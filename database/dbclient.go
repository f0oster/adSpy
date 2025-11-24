package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBClient struct {
	pool *pgxpool.Pool
}

func NewDBClient(pool *pgxpool.Pool) *DBClient {
	return &DBClient{
		pool: pool,
	}
}

// Returns the current version ID (nil if this is a new object).
func (r *DBClient) UpsertObject(
	ctx context.Context,
	tx pgx.Tx,
	objectID uuid.UUID,
	objectType string,
	dn string,
	domainID uuid.UUID,
) (*uuid.UUID, error) {
	var currentVersion *uuid.UUID
	err := tx.QueryRow(ctx, UpsertObject, objectID, objectType, dn, domainID).Scan(&currentVersion)
	if err != nil {
		return nil, fmt.Errorf("upsert object query failed: %w", err)
	}
	return currentVersion, nil
}

func (r *DBClient) GetVersionSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	versionID uuid.UUID,
) ([]byte, error) {
	var snapshotJSON []byte
	err := tx.QueryRow(ctx, GetPreviousSnapshot, versionID).Scan(&snapshotJSON)
	if err != nil {
		return nil, fmt.Errorf("get version snapshot query failed: %w", err)
	}
	return snapshotJSON, nil
}

func (r *DBClient) CreateVersion(
	ctx context.Context,
	tx pgx.Tx,
	versionID uuid.UUID,
	objectID uuid.UUID,
	timestamp time.Time,
	attributesJSON []byte,
	modifiedBy string,
) error {
	_, err := tx.Exec(ctx, InsertVersion,
		versionID,
		objectID,
		timestamp,
		attributesJSON,
		modifiedBy,
	)
	if err != nil {
		return fmt.Errorf("create version query failed: %w", err)
	}
	return nil
}

func (r *DBClient) UpdateCurrentVersion(
	ctx context.Context,
	tx pgx.Tx,
	versionID uuid.UUID,
	objectID uuid.UUID,
) error {
	_, err := tx.Exec(ctx, UpdateCurrentVersion, versionID, objectID)
	if err != nil {
		return fmt.Errorf("update current version query failed: %w", err)
	}
	return nil
}

func (r *DBClient) RecordAttributeChange(
	ctx context.Context,
	tx pgx.Tx,
	change *ChangeRecord,
) error {
	_, err := tx.Exec(ctx, InsertAttributeChange,
		change.ChangeID,
		change.ObjectID,
		change.AttributeName,
		change.OldValue,
		change.NewValue,
		change.VersionID,
		change.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("record attribute change query failed: %w", err)
	}
	return nil
}

func (r *DBClient) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction failed: %w", err)
	}
	return tx, nil
}

func (r *DBClient) CommitTx(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction failed: %w", err)
	}
	return nil
}

func (r *DBClient) RollbackTx(ctx context.Context, tx pgx.Tx) error {
	// Rollback returns an error if transaction is already committed/rolled back
	return tx.Rollback(ctx)
}
