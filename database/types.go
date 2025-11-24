package database

import (
	"time"

	"github.com/google/uuid"
)

// ObjectRecord represents a row in the Objects table.
// It tracks the basic metadata for an Active Directory object.
type ObjectRecord struct {
	ObjectID          uuid.UUID
	ObjectType        string
	DistinguishedName string
	DomainID          uuid.UUID
	CurrentVersion    *uuid.UUID // Nullable - points to current ObjectVersion
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time // Nullable
}

// VersionRecord represents a row in the ObjectVersions table.
// It stores a point-in-time snapshot of an object's attributes.
type VersionRecord struct {
	VersionID          uuid.UUID
	ObjectID           uuid.UUID
	Timestamp          time.Time
	AttributesSnapshot []byte // JSON blob of attributes
	ModifiedBy         string
}

// ChangeRecord represents a row in the AttributeChanges table.
// It tracks individual attribute modifications between versions.
type ChangeRecord struct {
	ChangeID      uuid.UUID
	ObjectID      uuid.UUID
	AttributeName string
	OldValue      []byte // JSON
	NewValue      []byte // JSON
	VersionID     uuid.UUID
	Timestamp     time.Time
}

