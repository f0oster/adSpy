package snapshot

import (
	"f0oster/adspy/activedirectory"
	"f0oster/adspy/diff"
	"fmt"
	"strconv"
	"time"
)

// Service handles snapshot creation and comparison for Active Directory objects.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

// CreateSnapshot converts an ActiveDirectoryObject into a Snapshot for storage.
// It extracts the object type (handling deleted objects specially) and builds
// a normalized attribute map suitable for database storage.
func (s *Service) CreateSnapshot(obj *activedirectory.ActiveDirectoryObject) (*Snapshot, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot create snapshot from nil object")
	}

	// Extract uSNChanged - this is required for versioning
	usnChangedStr, ok := obj.GetNormalizedAttribute("uSNChanged")
	if !ok {
		return nil, fmt.Errorf("object %s is missing required uSNChanged attribute", obj.DN)
	}
	usnChanged, err := strconv.ParseInt(usnChangedStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse uSNChanged value '%s': %w", usnChangedStr, err)
	}

	// Extract object type, handling deleted objects specially
	objectType := extractObjectType(obj)

	// Build normalized attributes map
	attributes := make(map[string][]string, len(obj.AttributeValues))
	for name, attr := range obj.AttributeValues {
		if attr.NormalizedValue != nil {
			attributes[name] = attr.NormalizedValue.Values
		}
	}

	// Check if object is deleted
	isDeleted := false
	if deletedAttr, ok := obj.GetNormalizedAttribute("isDeleted"); ok && deletedAttr == "TRUE" {
		isDeleted = true
	}

	return &Snapshot{
		ObjectGUID: obj.ObjectGUID,
		ObjectType: objectType,
		DN:         obj.DN,
		IsDeleted:  isDeleted,
		USNChanged: usnChanged,
		Attributes: attributes,
		Timestamp:  time.Now(),
	}, nil
}

// CompareSnapshots compares two attribute maps and returns the changes.
// It wraps the existing diff.FindChanges logic for consistency.
func (s *Service) CompareSnapshots(oldAttributes, newAttributes map[string][]string) []diff.AttributeChange {
	// Convert to map[string]interface{} for diff.FindChanges compatibility
	oldMap := make(map[string]interface{}, len(oldAttributes))
	for k, v := range oldAttributes {
		oldMap[k] = v
	}

	newMap := make(map[string]interface{}, len(newAttributes))
	for k, v := range newAttributes {
		newMap[k] = v
	}

	return diff.FindChanges(oldMap, newMap)
}

// extractObjectType determines the object type from an ActiveDirectoryObject.
// For deleted objects (missing objectCategory), it returns "deletedObject".
// Otherwise, it returns the normalized objectCategory value.
func extractObjectType(obj *activedirectory.ActiveDirectoryObject) string {
	// Try to get objectCategory first
	if objectCategory, ok := obj.GetNormalizedAttribute("objectCategory"); ok {
		return objectCategory
	}

	// Check if this is a deleted object
	if isDeleted, ok := obj.GetNormalizedAttribute("isDeleted"); ok && isDeleted == "TRUE" {
		return "deletedObject"
	}

	// Fallback to "unknown" (shouldn't happen in practice)
	return "unknown"
}
