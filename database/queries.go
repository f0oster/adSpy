package database

// SQL query constants for database operations.
// These queries are designed to be sqlc-compatible for future migration.

const (
	// Returns the current_version (NULL for new objects).
	UpsertObject = `
		INSERT INTO Objects (object_id, object_type, distinguishedName, domain_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (object_id)
		DO UPDATE SET
			updated_at = NOW(),
			distinguishedName = EXCLUDED.distinguishedName,
			object_type = EXCLUDED.object_type
		RETURNING current_version`

	InsertVersion = `
		INSERT INTO ObjectVersions (version_id, object_id, timestamp, attributes_snapshot, modified_by)
		VALUES ($1, $2, $3, $4, $5)`

	UpdateCurrentVersion = `
		UPDATE Objects
		SET current_version = $1
		WHERE object_id = $2`

	GetPreviousSnapshot = `
		SELECT attributes_snapshot
		FROM ObjectVersions
		WHERE version_id = $1`

	InsertAttributeChange = `
		INSERT INTO AttributeChanges (
			change_id,
			object_id,
			attribute_name,
			old_value,
			new_value,
			version_id,
			timestamp
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
)
