package database

import (
	"context"
	"fmt"
	"log"

	"f0oster/adspy/activedirectory"

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

func (db *Database) Connect() {
	var err error
	db.ConnectionPool, err = pgxpool.New(db.ctx, db.dsn)
	if err != nil {
		fmt.Printf("Unable to connect: %v\n", err)
		return
	}
}

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
