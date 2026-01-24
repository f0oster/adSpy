package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	dsn           string
	managementDsn string
	pool          *pgxpool.Pool
	client        *DBClient
}

func NewDatabase(dsn string, managementDsn string) *Database {
	return &Database{
		dsn:           dsn,
		managementDsn: managementDsn,
	}
}

func (db *Database) Connect(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, db.dsn)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	db.pool = pool
	db.client = NewDBClient(pool)
	return nil
}

func (db *Database) Client() *DBClient {
	return db.client
}

func (db *Database) Pool() *pgxpool.Pool {
	return db.pool
}

func (db *Database) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}
