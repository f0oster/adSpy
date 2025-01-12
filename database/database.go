package database

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
	    schema_metadata JSONB NOT NULL,
	    last_sync TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE Objects (
	    object_id UUID PRIMARY KEY,
	    object_type VARCHAR(255) NOT NULL,
	    current_version UUID,
	    domain_id UUID,
	    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    deleted_at TIMESTAMP
	);

	CREATE TABLE ObjectVersions (
	    version_id UUID PRIMARY KEY,
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
