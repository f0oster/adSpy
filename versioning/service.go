package versioning

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"f0oster/adspy/database"
	"f0oster/adspy/diff"
	"f0oster/adspy/snapshot"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Service handles versioning business logic for Active Directory objects.
// It orchestrates snapshot comparison, version creation, and change tracking.
type Service struct {
	dbClient        *database.DBClient
	snapshotService *snapshot.Service
}

func NewService(client *database.DBClient, snapSvc *snapshot.Service) *Service {
	return &Service{
		dbClient:        client,
		snapshotService: snapSvc,
	}
}

// ProcessSnapshots persists a batch of snapshots using versioning logic.
// It implements a fail-fast batch transaction strategy:
// - Single transaction for entire batch
// - Stops on first error and rolls back
// - All snapshots succeed or all fail together
func (s *Service) ProcessSnapshots(
	ctx context.Context,
	snapshots []*snapshot.Snapshot,
	domainID uuid.UUID,
) error {
	if len(snapshots) == 0 {
		return nil // Nothing to do
	}

	// Begin transaction for entire batch
	tx, err := s.dbClient.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer s.dbClient.RollbackTx(ctx, tx) // No-op if already committed

	// Process each snapshot
	for i, snap := range snapshots {
		if err := s.processSnapshot(ctx, tx, snap, domainID); err != nil {
			// Fail fast - rollback handled by defer
			return fmt.Errorf("failed to process snapshot %d (DN: %s): %w", i, snap.DN, err)
		}
	}

	// Commit entire batch
	if err := s.dbClient.CommitTx(ctx, tx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully processed %d snapshots", len(snapshots))
	return nil
}

// processSnapshot handles a single snapshot within a transaction.
// Business logic: Determines whether this is a new object or an update.
func (s *Service) processSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	snap *snapshot.Snapshot,
	domainID uuid.UUID,
) error {
	// Upsert the object record
	currentVersion, err := s.dbClient.UpsertObject(
		ctx, tx,
		snap.ObjectGUID,
		snap.ObjectType,
		snap.DN,
		domainID,
	)
	if err != nil {
		return fmt.Errorf("upsert object failed: %w", err)
	}

	// Business decision: New object (no current version) or existing object?
	if currentVersion == nil {
		return s.createInitialVersion(ctx, tx, snap)
	}

	// Existing object - check for changes
	return s.updateIfChanged(ctx, tx, snap, *currentVersion)
}

// createInitialVersion creates the first version for a new object.
// Business logic: New objects always get an initial version.
func (s *Service) createInitialVersion(
	ctx context.Context,
	tx pgx.Tx,
	snap *snapshot.Snapshot,
) error {
	// Marshal attributes to JSON
	snapshotJSON, err := s.marshalAttributes(snap.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	// Create new version
	newVersionID := uuid.New()
	if err := s.dbClient.CreateVersion(
		ctx, tx,
		newVersionID,
		snap.ObjectGUID,
		snap.Timestamp,
		snapshotJSON,
		ModifiedBySystem,
	); err != nil {
		return fmt.Errorf("failed to create version: %w", err)
	}

	// Update object to point to this version
	if err := s.dbClient.UpdateCurrentVersion(ctx, tx, newVersionID, snap.ObjectGUID); err != nil {
		return fmt.Errorf("failed to update current version: %w", err)
	}

	log.Printf("Created new object %s (DN: %s)", snap.ObjectGUID, snap.DN)
	return nil
}

// updateIfChanged checks if the snapshot differs from the previous version.
// Business logic: Only create a new version if attributes actually changed.
func (s *Service) updateIfChanged(
	ctx context.Context,
	tx pgx.Tx,
	snap *snapshot.Snapshot,
	currentVersionID uuid.UUID,
) error {
	// Load previous snapshot from database
	previousJSON, err := s.dbClient.GetVersionSnapshot(ctx, tx, currentVersionID)
	if err != nil {
		return fmt.Errorf("failed to load previous snapshot: %w", err)
	}

	// Unmarshal previous attributes
	previousAttributes, err := s.unmarshalAttributes(previousJSON)
	if err != nil {
		return fmt.Errorf("failed to unmarshal previous snapshot: %w", err)
	}

	// Business logic: Compare snapshots to detect changes
	changes, err := s.detectChanges(previousAttributes, snap.Attributes)
	if err != nil {
		return fmt.Errorf("failed to detect changes: %w", err)
	}

	// Business decision: No changes = no new version needed
	if len(changes) == 0 {
		return nil
	}

	// Marshal new snapshot
	snapshotJSON, err := s.marshalAttributes(snap.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	// Create new version
	newVersionID := uuid.New()
	if err := s.dbClient.CreateVersion(
		ctx, tx,
		newVersionID,
		snap.ObjectGUID,
		snap.Timestamp,
		snapshotJSON,
		ModifiedBySystem,
	); err != nil {
		return fmt.Errorf("failed to create new version: %w", err)
	}

	// Update current version pointer
	if err := s.dbClient.UpdateCurrentVersion(ctx, tx, newVersionID, snap.ObjectGUID); err != nil {
		return fmt.Errorf("failed to update current version: %w", err)
	}

	// Record individual attribute changes
	for _, change := range changes {
		oldJSON, err := json.Marshal(change.Old)
		if err != nil {
			return fmt.Errorf("failed to marshal old value for %s: %w", change.Name, err)
		}

		newJSON, err := json.Marshal(change.New)
		if err != nil {
			return fmt.Errorf("failed to marshal new value for %s: %w", change.Name, err)
		}

		if err := s.dbClient.RecordAttributeChange(
			ctx, tx,
			uuid.New(),
			snap.ObjectGUID,
			change.Name,
			oldJSON,
			newJSON,
			newVersionID,
			snap.Timestamp,
		); err != nil {
			return fmt.Errorf("failed to record attribute change for %s: %w", change.Name, err)
		}

		log.Printf("Attribute change for %s (DN: %s) - %s: %v -> %v",
			snap.ObjectGUID, snap.DN, change.Name, change.Old, change.New)
	}

	log.Printf("Updated object %s (DN: %s) with %d changes", snap.ObjectGUID, snap.DN, len(changes))
	return nil
}

// detectChanges uses the snapshot service to compare old and new attributes.
// Returns a list of detected attribute changes.
func (s *Service) detectChanges(
	previousAttributes map[string][]string,
	currentAttributes map[string][]string,
) ([]diff.AttributeChange, error) {
	// Use snapshot service for comparison logic
	changes := s.snapshotService.CompareSnapshots(previousAttributes, currentAttributes)
	return changes, nil
}

// marshalAttributes converts attribute map to JSON bytes.
func (s *Service) marshalAttributes(attributes map[string][]string) ([]byte, error) {
	return json.Marshal(attributes)
}

// unmarshalAttributes converts JSON bytes to attribute map.
// Handles type conversion from map[string]interface{} to map[string][]string.
func (s *Service) unmarshalAttributes(jsonData []byte) (map[string][]string, error) {
	var rawAttributes map[string]interface{}
	if err := json.Unmarshal(jsonData, &rawAttributes); err != nil {
		return nil, err
	}

	// Convert to expected map[string][]string format
	return convertToStringMap(rawAttributes), nil
}

// convertToStringMap converts a map[string]interface{} to map[string][]string.
// This handles the type conversion needed after JSON unmarshaling.
func convertToStringMap(m map[string]interface{}) map[string][]string {
	result := make(map[string][]string, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case []interface{}:
			strSlice := make([]string, len(val))
			for i, item := range val {
				if s, ok := item.(string); ok {
					strSlice[i] = s
				}
			}
			result[k] = strSlice
		case []string:
			result[k] = val
		case string:
			result[k] = []string{val}
		}
	}
	return result
}
