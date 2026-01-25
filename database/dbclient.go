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

// Returns the last processed USN (nil if this is a new object).
func (r *DBClient) UpsertObject(
	ctx context.Context,
	tx pgx.Tx,
	objectID uuid.UUID,
	objectType string,
	dn string,
	domainID uuid.UUID,
) (*int64, error) {
	txQueries := r.queries.WithTx(tx)

	currentUSN, err := txQueries.UpsertObject(ctx, sqlcgen.UpsertObjectParams{
		ObjectID:          uuidToPgtype(objectID),
		ObjectType:        objectType,
		Distinguishedname: dn,
		DomainID:          uuidToPgtype(domainID),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert object query failed: %w", err)
	}

	return pgtypeToInt64(currentUSN), nil
}

func (r *DBClient) GetVersionSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	objectID uuid.UUID,
	usnChanged int64,
) ([]byte, error) {
	txQueries := r.queries.WithTx(tx)

	snapshotJSON, err := txQueries.GetPreviousSnapshot(ctx, sqlcgen.GetPreviousSnapshotParams{
		ObjectID:   uuidToPgtype(objectID),
		UsnChanged: usnChanged,
	})
	if err != nil {
		return nil, fmt.Errorf("get version snapshot query failed: %w", err)
	}
	return snapshotJSON, nil
}

func (r *DBClient) CreateVersion(
	ctx context.Context,
	tx pgx.Tx,
	objectID uuid.UUID,
	usnChanged int64,
	timestamp time.Time,
	attributesJSON []byte,
	modifiedBy string,
) error {
	txQueries := r.queries.WithTx(tx)

	err := txQueries.InsertVersion(ctx, sqlcgen.InsertVersionParams{
		ObjectID:           uuidToPgtype(objectID),
		UsnChanged:         usnChanged,
		Timestamp:          pgtype.Timestamp{Time: timestamp, Valid: true},
		AttributesSnapshot: attributesJSON,
		ModifiedBy:         pgtype.Text{String: modifiedBy, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create version query failed: %w", err)
	}
	return nil
}

func (r *DBClient) UpdateLastProcessedUSN(
	ctx context.Context,
	tx pgx.Tx,
	usnChanged int64,
	objectID uuid.UUID,
) error {
	txQueries := r.queries.WithTx(tx)

	err := txQueries.UpdateLastProcessedUSN(ctx, sqlcgen.UpdateLastProcessedUSNParams{
		LastProcessedUsn: pgtype.Int8{Int64: usnChanged, Valid: true},
		ObjectID:         uuidToPgtype(objectID),
	})
	if err != nil {
		return fmt.Errorf("update last processed USN query failed: %w", err)
	}
	return nil
}

func (r *DBClient) RecordAttributeChange(
	ctx context.Context,
	tx pgx.Tx,
	objectID uuid.UUID,
	usnChanged int64,
	attributeSchemaID uuid.UUID,
	oldValue []byte,
	newValue []byte,
	timestamp time.Time,
) error {
	txQueries := r.queries.WithTx(tx)

	err := txQueries.InsertAttributeChange(ctx, sqlcgen.InsertAttributeChangeParams{
		ObjectID:          uuidToPgtype(objectID),
		UsnChanged:        usnChanged,
		AttributeSchemaID: uuidToPgtype(attributeSchemaID),
		OldValue:          oldValue,
		NewValue:          newValue,
		Timestamp:         pgtype.Timestamp{Time: timestamp, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("record attribute change query failed: %w", err)
	}
	return nil
}

func (r *DBClient) GetAttributeSchemaByLDAPName(
	ctx context.Context,
	domainID uuid.UUID,
	ldapDisplayName string,
) (*uuid.UUID, error) {
	schemaGUID, err := r.queries.GetAttributeSchemaByLDAPName(ctx, sqlcgen.GetAttributeSchemaByLDAPNameParams{
		DomainID:        uuidToPgtype(domainID),
		LdapDisplayName: ldapDisplayName,
	})
	if err != nil {
		return nil, fmt.Errorf("get attribute schema by LDAP name query failed: %w", err)
	}
	return pgtypeToUUID(schemaGUID), nil
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
		LastProcessedUsn: pgtype.Int8{Int64: 0, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert domain failed: %w", err)
	}
	return nil
}

func (r *DBClient) UpsertAttributeSchema(
	ctx context.Context,
	objectGUID uuid.UUID,
	domainID uuid.UUID,
	ldapDisplayName string,
	attributeName string,
	attributeID string,
	attributeSyntax string,
	omSyntax string,
	syntaxName string,
	isSingleValued bool,
) error {
	err := r.queries.UpsertAttributeSchema(ctx, sqlcgen.UpsertAttributeSchemaParams{
		ObjectGuid:      uuidToPgtype(objectGUID),
		DomainID:        uuidToPgtype(domainID),
		LdapDisplayName: ldapDisplayName,
		AttributeName:   attributeName,
		AttributeID:     attributeID,
		AttributeSyntax: attributeSyntax,
		OmSyntax:        omSyntax,
		SyntaxName:      pgtype.Text{String: syntaxName, Valid: syntaxName != ""},
		IsSingleValued:  isSingleValued,
	})
	if err != nil {
		return fmt.Errorf("upsert attribute schema failed: %w", err)
	}
	return nil
}

func (r *DBClient) UpdateDomainLastProcessedUSN(
	ctx context.Context,
	domainID uuid.UUID,
	lastProcessedUSN int64,
) error {
	err := r.queries.UpdateDomainLastProcessedUSN(ctx, sqlcgen.UpdateDomainLastProcessedUSNParams{
		DomainID:         uuidToPgtype(domainID),
		LastProcessedUsn: pgtype.Int8{Int64: lastProcessedUSN, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update domain last processed USN failed: %w", err)
	}
	return nil
}

func (r *DBClient) UpdateDomainHighestUSN(
	ctx context.Context,
	domainID uuid.UUID,
	highestUSN int64,
) error {
	err := r.queries.UpdateDomainHighestUSN(ctx, sqlcgen.UpdateDomainHighestUSNParams{
		DomainID:   uuidToPgtype(domainID),
		HighestUsn: pgtype.Int8{Int64: highestUSN, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update domain highest USN failed: %w", err)
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

func pgtypeToInt64(val pgtype.Int8) *int64 {
	if !val.Valid {
		return nil
	}
	return &val.Int64
}
