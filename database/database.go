package database

import (
	"context"
	"encoding/json"
	"f0oster/adspy/activedirectory"
	"f0oster/adspy/diff"
	"fmt"
	"log"
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
func (db *Database) InitalizeDomain(adInstance *activedirectory.ActiveDirectoryInstance) error {

	// TODO: we'll later support loading an existing domain and mapping schema changes, for now, let's just use a static GUID and disregard schema history entirely
	adInstance.DomainId, _ = uuid.Parse("4ee698a0-e182-45f1-834d-019fd66a1ceb") // for now, use a static domain GUID
	// adInstance.DomainId = uuid.New()

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

func (db *Database) WriteObjects(adInstance *activedirectory.ActiveDirectoryInstance, entries []*ldap.Entry) error {
	tx, err := db.ConnectionPool.Begin(db.ctx)
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

		// Fetch and format objectGUID
		objectGUID, err := adObject.GetObjectGUID()
		if err != nil {
			log.Printf("Failed to get objectGUID for DN %s: %v\n", entry.DN, err)
			continue
		}

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

		objectSID, ok := adObject.GetNormalizedAttribute("objectSid")
		if ok {
			log.Printf("ObjectSID: %s (%s)\n", objectSID, objectCategory)
		} else {
			log.Printf("Failed to get objectSID for DN %s (%s)\n", entry.DN, objectCategory)
		}

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
			db.ctx,
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
				db.ctx,
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

			_, err = tx.Exec(db.ctx, updateObjectVersionQuery, newVersionID, objectGUID)
			if err != nil {
				return fmt.Errorf("failed to update current version: %w", err)
			}

			log.Printf("Created new object (%s) and version for DN %s", stringObjectID, entry.DN)
			continue
		}

		// Load existing snapshot
		var existingJSON []byte
		err = tx.QueryRow(db.ctx, `
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
			fmt.Println("No changes detected")
			continue
		}

		newVersionID := uuid.New()
		_, err = tx.Exec(
			db.ctx,
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

		_, err = tx.Exec(db.ctx, updateObjectVersionQuery, newVersionID, objectGUID)
		if err != nil {
			return fmt.Errorf("failed to update current version: %w", err)
		}

		log.Printf("+++Updated object (%s) for DN %s", stringObjectID, entry.DN)
		for _, ch := range changed {
			log.Printf("+++Attr change: %s: %v -> %v", ch.Name, ch.Old, ch.New)

			// Marshal old and new values to JSONB
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
				db.ctx,
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

func ResetDatabase(ctx context.Context) {

	managementDsn := "postgres://postgres:example@dockerprdap01:5432/postgres"

	managementPool, err := pgxpool.New(context.Background(), managementDsn)
	if err != nil {
		fmt.Printf("Unable to connect: %v\n", err)
		return
	}
	defer managementPool.Close()

	_, err = managementPool.Exec(ctx, "DROP DATABASE IF EXISTS adspy")
	if err != nil {
		log.Fatalf("Failed to drop database: %v", err)
	}
	fmt.Println("Database 'adspy' dropped successfully (if it existed).")

	_, err = managementPool.Exec(ctx, "CREATE DATABASE adspy")
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	fmt.Println("Database 'adspy' created successfully.")

	managementPool.Close()

	adSpyDsn := "postgres://postgres:example@dockerprdap01:5432/adspy"

	adSpyPool, err := pgxpool.New(context.Background(), adSpyDsn)
	if err != nil {
		fmt.Printf("Unable to connect: %v\n", err)
		return
	}
	defer adSpyPool.Close()

	createTablesSQL := `
	CREATE TABLE Domains (
		domain_id UUID PRIMARY KEY,
		domain_name VARCHAR(255) NOT NULL,
		schema_metadata JSONB,
		domain_controller VARCHAR NOT NULL,
		current_usn BIGINT,
		highest_usn BIGINT
	);

	CREATE TABLE Objects (
	    object_id UUID PRIMARY KEY,
	    object_type VARCHAR(255) NOT NULL,
		distinguishedName VARCHAR(255),
	    current_version UUID,
	    domain_id UUID,
	    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    deleted_at TIMESTAMP
	);

	CREATE TABLE ObjectVersions (
	    version_id UUID PRIMARY KEY NOT NULL,
	    object_id UUID NOT NULL,
	    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    attributes_snapshot JSONB NOT NULL,
	    modified_by VARCHAR(255)
	);

	CREATE TABLE AttributeChanges (
	    change_id UUID PRIMARY KEY,
	    object_id UUID NOT NULL,
	    attribute_name VARCHAR(255) NOT NULL,
	    old_value JSONB,
	    new_value JSONB,
	    version_id UUID NOT NULL,
	    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	ALTER TABLE Objects
	ADD CONSTRAINT fk_objects_current_version FOREIGN KEY (current_version) REFERENCES ObjectVersions(version_id),
	ADD CONSTRAINT fk_objects_domain_id FOREIGN KEY (domain_id) REFERENCES Domains(domain_id);

	ALTER TABLE ObjectVersions
	ADD CONSTRAINT fk_object_versions_object_id FOREIGN KEY (object_id) REFERENCES Objects(object_id);

	ALTER TABLE AttributeChanges
	ADD CONSTRAINT fk_attribute_changes_object_id FOREIGN KEY (object_id) REFERENCES Objects(object_id),
	ADD CONSTRAINT fk_attribute_changes_version_id FOREIGN KEY (version_id) REFERENCES ObjectVersions(version_id);
	`
	_, err = adSpyPool.Exec(ctx, createTablesSQL)
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
	fmt.Println("Tables created successfully.")
}
