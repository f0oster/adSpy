package database

import (
	"context"
	"encoding/json"
	"f0oster/adspy/activedirectory"
	"f0oster/adspy/activedirectory/formatters"
	"f0oster/adspy/activedirectory/ldaphelpers"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
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

// TODO: Implement uniform serialization/deserialization
func normalizeSlice(value interface{}) ([]interface{}, bool) {

	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice {
		return nil, false
	}

	length := v.Len()
	// Convert the slice elements to []interface{}
	result := make([]interface{}, length)
	for i := 0; i < length; i++ {
		result[i] = v.Index(i).Interface()
	}

	return result, true
}

// TODO: Implement uniform serialization/deserialization so normalizeSlice isn't necessary
func findChanges(LiveADAttributes, DatabaseAttributes map[string]interface{}) []map[string]interface{} {
	var changes []map[string]interface{}

	for key, newValue := range LiveADAttributes {
		oldValue, exists := DatabaseAttributes[key]

		reason := ""

		if !exists {
			reason = "Key does not exist in the DatabaseAttributes"
		} else {
			// Normalize slices for comparison
			newSlice, newIsSlice := normalizeSlice(newValue)
			oldSlice, oldIsSlice := normalizeSlice(oldValue)

			if newIsSlice && oldIsSlice {
				if !reflect.DeepEqual(newSlice, oldSlice) {
					reason = "Value does not match."
				} else {
					// fmt.Printf("No change detected for: %s\n", key)
					continue
				}
			} else if !reflect.DeepEqual(newValue, oldValue) {
				reason = "Value does not match."
			} else {
				// fmt.Printf("No change detected for: %s\n", key)
				continue
			}
		}

		changes = append(changes, map[string]interface{}{
			"attribute_name": key,
			"old_value":      oldValue,
			"new_value":      newValue,
			"reason":         reason,
		})

		fmt.Printf("Change detected for %s: %s\n", key, reason)
	}

	return changes
}

// TODO: refactor & clean - WIP, initial proof of concept
func (db *Database) WriteObjects(adInstance *activedirectory.ActiveDirectoryInstance, entries []*ldap.Entry) error {
	tx, err := db.ConnectionPool.Begin(db.ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback(db.ctx)
			panic(p)
		} else if err != nil {
			tx.Rollback(db.ctx)
		} else {
			err = tx.Commit(db.ctx)
		}
	}()

	// TODO: on conflict update all attribs, besides object_id, domain_id and current_version?
	insertObjectQuery := `
	INSERT INTO Objects (object_id, object_type, distinguishedName, domain_id)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (object_id) 
	DO UPDATE SET updated_at = EXCLUDED.updated_at
	RETURNING current_version;
    `
	insertObjectVersionQuery := `
        INSERT INTO ObjectVersions (version_id, object_id, timestamp, attributes_snapshot, modified_by)
        VALUES ($1, $2, $3, $4, $5)
    `
	insertAttributeChangesQuery := `
        INSERT INTO AttributeChanges (change_id, version_id, object_id, timestamp, attribute_name, old_value, new_value)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `
	updateObjectVersionQuery := `
        UPDATE Objects
        SET current_version = $1
        WHERE object_id = $2
    `

	requiredAttrs := []string{"objectGUID", "objectCategory"}

	for _, entry := range entries {
		attributes, err := ldaphelpers.ExtractAttributes(entry, requiredAttrs)
		if err != nil {
			log.Printf("Skipping entry for DN %s: %v", entry.DN, err)
			continue
		}

		objectID, err := formatters.FormatObjectGUID([]byte(attributes["objectGUID"]))
		if err != nil {
			log.Printf("Failed to format objectGUID for DN %s: %v", entry.DN, err)
			continue
		}

		createdAt := time.Now()

		var currentVersion *string
		err = tx.QueryRow(db.ctx, insertObjectQuery, objectID, attributes["objectCategory"], entry.DN, adInstance.DomainId).Scan(&currentVersion)
		if err != nil {
			log.Printf("Failed to insert object for DN %s: %v", entry.DN, err)
			return fmt.Errorf("failed to insert object for DN %s: %w", entry.DN, err)
		}

		// Serialize attributes for the object version
		LiveADAttributeSnapshot, err := ldaphelpers.SerializeAttributes(entry, adInstance)
		if err != nil {
			log.Printf("Failed to serialize attributes for DN %s: %v", entry.DN, err)
			continue
		}

		LiveADAttributeSnapshotJSON, err := json.Marshal(LiveADAttributeSnapshot)
		if err != nil {
			log.Printf("Failed to serialize attributes to JSON for DN %s: %v", entry.DN, err)
			return fmt.Errorf("failed to serialize attributes to JSON for DN %s: %w", entry.DN, err)
		}

		// If version differs, or if there are no versions yet, let's create a new version
		if currentVersion == nil {
			// No version yet - let's make one
			log.Printf("Object for DN %s was just created, no current version set yet.", entry.DN)

			// Insert the object version
			objectVersionID := uuid.New()
			log.Printf("Inserting object version for %s", entry.DN)
			_, err = tx.Exec(db.ctx, insertObjectVersionQuery, objectVersionID, objectID, createdAt, LiveADAttributeSnapshotJSON, "system")
			if err != nil {
				log.Printf("Failed to insert object version for DN %s: %v", entry.DN, err)
				return fmt.Errorf("failed to insert object version for DN %s: %w", entry.DN, err)
			}

			// Update the current_version for the object
			log.Printf("Setting current object version for %s", entry.DN)
			_, err = tx.Exec(db.ctx, updateObjectVersionQuery, objectVersionID, objectID)
			if err != nil {
				log.Printf("Failed to set current object version for DN %s: %v", entry.DN, err)
				return fmt.Errorf("failed to set object version for DN %s: %w", entry.DN, err)
			}
		} else {
			// there's a corresponding ObjectVersion - let's compare for changes
			log.Printf("Current version for DN %s: %v", entry.DN, *currentVersion)
			log.Printf("Fetching current version to compare changes...")

			// Fetch the latest ObjectVersion
			var existingDatabaseAttributesSnapshotJSON []byte
			err = tx.QueryRow(db.ctx, `
					SELECT attributes_snapshot, current_version
					FROM Objects
					JOIN ObjectVersions ON Objects.current_version = ObjectVersions.version_id
					WHERE Objects.object_id = $1
				`, objectID).Scan(&existingDatabaseAttributesSnapshotJSON, &currentVersion)

			if err != nil {
				log.Fatalf("select snapshot error: %v", err)
			}

			var existingDatabaseAttributeSnapshot map[string]interface{}
			err = json.Unmarshal(existingDatabaseAttributesSnapshotJSON, &existingDatabaseAttributeSnapshot)
			if err != nil {
				log.Fatal(err)
			}

			// Compare the live AD values with the DB snapshot
			mismatchedAttrs := findChanges(LiveADAttributeSnapshot, existingDatabaseAttributeSnapshot)

			// If there are changes, we create a new objectversion
			if len(mismatchedAttrs) > 0 {
				log.Printf("Changes detected on attributes for DN %s: %v", entry.DN, mismatchedAttrs)
				objectVersionID := uuid.New()
				log.Printf("Inserting new object version for %s", entry.DN)
				_, err = tx.Exec(db.ctx, insertObjectVersionQuery, objectVersionID, objectID, createdAt, LiveADAttributeSnapshotJSON, "system")
				if err != nil {
					log.Printf("Failed to insert object version for DN %s: %v", entry.DN, err)
					return fmt.Errorf("failed to insert object version for DN %s: %w", entry.DN, err)
				}

				// Handle each attribute mismatch
				for key, value := range mismatchedAttrs {
					log.Printf("Writing AttributeChange for DN %s: %v", entry.DN, key)
					_, err = tx.Exec(db.ctx, insertAttributeChangesQuery, uuid.New(), objectVersionID, objectID, createdAt, value["attribute_name"], value["old_value"], value["new_value"])
					if err != nil {
						log.Printf("Failed to insert AttributeChange: %v", err)
						return fmt.Errorf("failed to insert AttributeChange: %w", err)
					}
				}

				// Update the current_version for the object
				log.Printf("Setting object current_version for %s", entry.DN)
				_, err = tx.Exec(db.ctx, updateObjectVersionQuery, objectVersionID, objectID)
				if err != nil {
					log.Printf("Failed to set current object version for DN %s: %v", entry.DN, err)
					return fmt.Errorf("failed to set object version for DN %s: %w", entry.DN, err)
				}
			} else {
				log.Printf("No changes detected for DN %s", entry.DN)
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
