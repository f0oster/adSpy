package database

import (
	"context"
	"encoding/json"
	"f0oster/adspy/activedirectory"
	"f0oster/adspy/activedirectory/formatters"
	"f0oster/adspy/activedirectory/ldaphelpers"
	"fmt"
	"log"
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

func (db *Database) WriteObjects(adInstance *activedirectory.ActiveDirectoryInstance, entries []*ldap.Entry) error {
	tx, err := db.ConnectionPool.Begin(db.ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback(db.ctx)
			panic(p) // Re-throw the panic
		} else if err != nil {
			tx.Rollback(db.ctx)
		} else {
			err = tx.Commit(db.ctx)
		}
	}()

	// Prepare SQL queries
	insertObjectQuery := `
        INSERT INTO Objects (object_id, object_type, distinguishedName, domain_id, created_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (object_id) DO NOTHING
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

		log.Printf("Upserting object for %s", entry.DN)
		_, err = tx.Exec(db.ctx, insertObjectQuery, objectID, attributes["objectCategory"], entry.DN, adInstance.DomainId, createdAt)
		if err != nil {
			log.Printf("Failed to insert object for DN %s: %v", entry.DN, err)
			return fmt.Errorf("failed to insert object for DN %s: %w", entry.DN, err)
		}

		attributesSnapshot, err := ldaphelpers.SerializeAttributes(entry, adInstance)
		if err != nil {
			log.Printf("Failed to serialize attributes for DN %s: %v", entry.DN, err)
			continue
		}

		// TODO: compare the attributes with current_version (if exists)

		attributesSnapshotJSON, err := json.Marshal(attributesSnapshot)
		if err != nil {
			log.Printf("Failed to serialize attributes to JSON for DN %s: %v", entry.DN, err)
			return fmt.Errorf("failed to serialize attributes to JSON for DN %s: %w", entry.DN, err)
		}

		// TODO: conditionally insert object version, only if current state differs from current version

		// Insert object version
		objectVersionID := uuid.New()
		log.Printf("Inserting object version for %s", entry.DN)
		_, err = tx.Exec(db.ctx, insertObjectVersionQuery, objectVersionID, objectID, createdAt, attributesSnapshotJSON, "system")
		if err != nil {
			log.Printf("Failed to insert object version for DN %s: %v", entry.DN, err)
			return fmt.Errorf("failed to insert object version for DN %s: %w", entry.DN, err)
		}

		// Update current object version
		log.Printf("Setting current object version for %s", entry.DN)
		_, err = tx.Exec(db.ctx, updateObjectVersionQuery, objectVersionID, objectID)
		if err != nil {
			log.Printf("Failed to set current object version for DN %s: %v", entry.DN, err)
			return fmt.Errorf("failed to set object version for DN %s: %w", entry.DN, err)
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
