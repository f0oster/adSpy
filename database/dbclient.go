package database

import (
	"context"
	"fmt"
	"time"

	"f0oster/adspy/database/sqlcgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBClient struct {
	pool    *pgxpool.Pool
	queries *sqlcgen.Queries
}

func NewDBClient(pool *pgxpool.Pool) *DBClient {
	return &DBClient{
		pool:    pool,
		queries: sqlcgen.New(pool),
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
	txQueries := r.queries.WithTx(tx)

	currentVersion, err := txQueries.UpsertObject(ctx, sqlcgen.UpsertObjectParams{
		ObjectID:          uuidToPgtype(objectID),
		ObjectType:        objectType,
		Distinguishedname: pgtype.Text{String: dn, Valid: true},
		DomainID:          uuidToPgtype(domainID),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert object query failed: %w", err)
	}

	return pgtypeToUUID(currentVersion), nil
}

func (r *DBClient) GetVersionSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	versionID uuid.UUID,
) ([]byte, error) {
	txQueries := r.queries.WithTx(tx)

	snapshotJSON, err := txQueries.GetPreviousSnapshot(ctx, uuidToPgtype(versionID))
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
	txQueries := r.queries.WithTx(tx)

	err := txQueries.InsertVersion(ctx, sqlcgen.InsertVersionParams{
		VersionID:          uuidToPgtype(versionID),
		ObjectID:           uuidToPgtype(objectID),
		Timestamp:          pgtype.Timestamp{Time: timestamp, Valid: true},
		AttributesSnapshot: attributesJSON,
		ModifiedBy:         pgtype.Text{String: modifiedBy, Valid: true},
	})
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
	txQueries := r.queries.WithTx(tx)

	err := txQueries.UpdateCurrentVersion(ctx, sqlcgen.UpdateCurrentVersionParams{
		CurrentVersion: uuidToPgtype(versionID),
		ObjectID:       uuidToPgtype(objectID),
	})
	if err != nil {
		return fmt.Errorf("update current version query failed: %w", err)
	}
	return nil
}

func (r *DBClient) RecordAttributeChange(
	ctx context.Context,
	tx pgx.Tx,
	changeID uuid.UUID,
	objectID uuid.UUID,
	attributeName string,
	oldValue []byte,
	newValue []byte,
	versionID uuid.UUID,
	timestamp time.Time,
) error {
	txQueries := r.queries.WithTx(tx)

	err := txQueries.InsertAttributeChange(ctx, sqlcgen.InsertAttributeChangeParams{
		ChangeID:      uuidToPgtype(changeID),
		ObjectID:      uuidToPgtype(objectID),
		AttributeName: attributeName,
		OldValue:      oldValue,
		NewValue:      newValue,
		VersionID:     uuidToPgtype(versionID),
		Timestamp:     pgtype.Timestamp{Time: timestamp, Valid: true},
	})
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

func (r *DBClient) InsertDomain(
	ctx context.Context,
	domainID uuid.UUID,
	domainName string,
	domainController string,
	highestUSN int64,
) error {
	err := r.queries.InsertDomain(ctx, sqlcgen.InsertDomainParams{
		DomainID:         uuidToPgtype(domainID),
		DomainName:       domainName,
		DomainController: domainController,
		HighestUsn:       pgtype.Int8{Int64: highestUSN, Valid: true},
		CurrentUsn:       pgtype.Int8{Int64: 0, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert domain failed: %w", err)
	}
	return nil
}

// Helper functions for UUID conversion

func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func pgtypeToUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	result := uuid.UUID(id.Bytes)
	return &result
}
