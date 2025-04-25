package database

import (
	"context"
	"encoding/json"
	"f0oster/adspy/activedirectory"
	"f0oster/adspy/diff"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	dsn            string
	managementDsn  string
	ConnectionPool *pgxpool.Pool
	ctx            context.Context
}

func NewDatabase(dsn string, managementDsn string, ctx context.Context) *Database {
	return &Database{
		dsn:           dsn,
		managementDsn: managementDsn,
		ctx:           ctx,
	}
}

// add a connection to the pgx connection pool
func (db *Database) Connect() {
	var err error
	db.ConnectionPool, err = pgxpool.New(db.ctx, db.dsn)
	if err != nil {
		fmt.Printf("Unable to connect: %v\n", err)
		return
	}
}

// initialize the domain
func (db *Database) InsertDomain(adInstance *activedirectory.ActiveDirectoryInstance) error {

	// TODO: we'll later support storing and mapping schema changes over time, for now disregard schema history
	_, err := db.ConnectionPool.Exec(db.ctx, `
		INSERT INTO domains (domain_id, domain_name, domain_controller, highest_usn, current_usn)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (domain_id) DO NOTHING
	`, adInstance.DomainId, adInstance.BaseDn, adInstance.DomainControllerFQDN, adInstance.HighestCommittedUSN, 0)

	if err != nil {
		log.Fatalf("insert object error: %v", err)
	}

	fmt.Printf("Initialized Domain: %v", adInstance.DomainId)
	return nil

}

func rollbackOrCommit(tx pgx.Tx, err *error) {
	if *err != nil {
		if rbErr := tx.Rollback(context.Background()); rbErr != nil {
			log.Printf("transaction rollback failed: %v (original error: %v)", rbErr, *err)
		} else {
			log.Printf("transaction rolled back due to error: %v", *err)
		}
	} else {
		if cmErr := tx.Commit(context.Background()); cmErr != nil {
			*err = fmt.Errorf("commit failed: %w", cmErr)
			log.Printf("transaction commit failed: %v", cmErr)
		}
	}
}

func (db *Database) WriteObjects(ctx context.Context, adInstance *activedirectory.ActiveDirectoryInstance, entries []*ldap.Entry) error {
	// TODO: refactor this mess. parsing shouldn't occur here - we should receive already processed model types here.
	conn, err := db.ConnectionPool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer rollbackOrCommit(tx, &err)

	insertObjectQuery := `
		INSERT INTO Objects (object_id, object_type, distinguishedName, domain_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (object_id)
		DO UPDATE SET updated_at = NOW()
		RETURNING current_version;
	`

	insertObjectVersionQuery := `
		INSERT INTO ObjectVersions (version_id, object_id, timestamp, attributes_snapshot, modified_by)
		VALUES ($1, $2, $3, $4, $5)
	`

	updateObjectVersionQuery := `
		UPDATE Objects
		SET current_version = $1
		WHERE object_id = $2
	`

	for _, entry := range entries {
		adObject, err := adInstance.ParseLDAPAttributeValues(entry)
		if err != nil {
			log.Printf("Skipping entry for DN %s: %v", entry.DN, err)
			continue
		}
		if len(adObject.AttributeValues) == 0 {
			continue
		}

		objectGUID := adObject.ObjectGUID

		stringObjectID, err := adObject.AttributeValues["objectGUID"].AsString()
		if err != nil {
			log.Printf("Failed to stringify objectGUID for DN %s: %v\n", entry.DN, err)
			continue
		}

		// objectCategory
		objectCategory, ok := adObject.GetNormalizedAttribute("objectCategory")
		if !ok {
			log.Printf("Failed to get objectCategory for DN %s\n", entry.DN)
			continue
		}

		// objectSID, ok := adObject.GetNormalizedAttribute("objectSid")
		// if ok {
		// 	log.Printf("ObjectSID: %s (%s)\n", objectSID, objectCategory)
		// } else {
		// 	log.Printf("Failed to get objectSID for DN %s (%s)\n", entry.DN, objectCategory)
		// }

		// Snapshot as map[string]interface{} (preserving []string where appropriate)
		attrSnapshot := make(map[string]interface{})
		for name, attr := range adObject.AttributeValues {
			val := attr.NormalizedValue.Values
			attrSnapshot[name] = val
		}

		snapshotJSON, err := json.Marshal(attrSnapshot)
		if err != nil {
			log.Printf("Failed to marshal snapshot for DN %s: %v\n", entry.DN, err)
			continue
		}

		var currentVersion *uuid.UUID
		err = tx.QueryRow(
			ctx,
			insertObjectQuery,
			objectGUID,
			objectCategory,
			entry.DN,
			adInstance.DomainId,
		).Scan(&currentVersion)
		if err != nil {
			return fmt.Errorf("failed to insert/update object: %w", err)
		}

		now := time.Now()

		if currentVersion == nil {
			newVersionID := uuid.New()
			_, err = tx.Exec(
				ctx,
				insertObjectVersionQuery,
				newVersionID,
				objectGUID,
				now,
				snapshotJSON,
				"system",
			)
			if err != nil {
				return fmt.Errorf("failed to insert initial version: %w", err)
			}

			_, err = tx.Exec(ctx, updateObjectVersionQuery, newVersionID, objectGUID)
			if err != nil {
				return fmt.Errorf("failed to update current version: %w", err)
			}

			log.Printf("Created new object (%s) and version for DN %s", stringObjectID, entry.DN)
			continue
		}

		// Load existing snapshot
		var existingJSON []byte
		err = tx.QueryRow(ctx, `
			SELECT attributes_snapshot
			FROM ObjectVersions
			WHERE version_id = $1
		`, currentVersion).Scan(&existingJSON)
		if err != nil {
			return fmt.Errorf("failed to load previous snapshot: %w", err)
		}

		var existing map[string]interface{}
		if err := json.Unmarshal(existingJSON, &existing); err != nil {
			return fmt.Errorf("failed to unmarshal existing snapshot: %w", err)
		}

		changed := diff.FindChanges(existing, attrSnapshot)
		if len(changed) == 0 {
			// fmt.Println("No changes detected")
			continue
		}

		newVersionID := uuid.New()
		_, err = tx.Exec(
			ctx,
			insertObjectVersionQuery,
			newVersionID,
			objectGUID,
			now,
			snapshotJSON,
			"system",
		)
		if err != nil {
			return fmt.Errorf("failed to insert changed version: %w", err)
		}

		_, err = tx.Exec(ctx, updateObjectVersionQuery, newVersionID, objectGUID)
		if err != nil {
			return fmt.Errorf("failed to update current version: %w", err)
		}

		for _, ch := range changed {
			log.Printf("   +++ Attr change for object (%s) DN: %s - %s: %v -> %v", stringObjectID, entry.DN, ch.Name, ch.Old, ch.New)

			oldJSON, err := json.Marshal(ch.Old)
			if err != nil {
				return fmt.Errorf("marshal old_value for %s: %w", ch.Name, err)
			}
			newJSON, err := json.Marshal(ch.New)
			if err != nil {
				return fmt.Errorf("marshal new_value for %s: %w", ch.Name, err)
			}

			const insertAttributeChangeQuery = `
			INSERT INTO AttributeChanges (
				change_id,
				object_id,
				attribute_name,
				old_value,
				new_value,
				version_id,
				timestamp
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			`

			_, err = tx.Exec(
				ctx,
				insertAttributeChangeQuery,
				uuid.New(),   // change_id
				objectGUID,   // object_id
				ch.Name,      // attribute_name
				oldJSON,      // old_value
				newJSON,      // new_value
				newVersionID, // version_id
				now,          // timestamp
			)
			if err != nil {
				return fmt.Errorf("insert AttributeChange for %s: %w", ch.Name, err)
			}
		}
	}

	return nil
}

func chunk(entries []*ldap.Entry, chunkSize int) [][]*ldap.Entry {
	var chunks [][]*ldap.Entry
	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}
		chunks = append(chunks, entries[i:end])
	}
	return chunks
}

func (db *Database) DispatchObjectWrites(adInstance *activedirectory.ActiveDirectoryInstance, entries []*ldap.Entry) error {
	chunks := chunk(entries, 100)
	var wg sync.WaitGroup
	errChan := make(chan error, len(chunks))

	for _, chunk := range chunks {
		wg.Add(1)
		go func(entries []*ldap.Entry) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			db.WriteObjects(ctx, adInstance, entries)

		}(chunk)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		return err
	}

	return nil
}
