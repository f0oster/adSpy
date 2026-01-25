package snapshot

import (
	"time"

	"github.com/google/uuid"
)

// Snapshot represents a point-in-time state of an Active Directory object.
// It contains all necessary information for storage and comparison.
type Snapshot struct {
	// ObjectGUID uniquely identifies the AD object
	ObjectGUID uuid.UUID

	// ObjectType is the category of the object (e.g., "user", "group", "deletedObject")
	ObjectType string

	// DN is the Distinguished Name of the object
	DN string

	// IsDeleted indicates if this object is in the Deleted Objects container
	IsDeleted bool

	// USNChanged is the AD-native update sequence number for this version
	USNChanged int64

	// Attributes contains the normalized string representation of all object attributes
	// Key: attribute name, Value: attribute values as string slice
	Attributes map[string][]string

	// Timestamp records when this snapshot was created
	Timestamp time.Time
}
