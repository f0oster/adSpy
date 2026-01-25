-- name: InsertAttributeChange :exec
INSERT INTO AttributeChanges (
    object_id,
    usn_changed,
    attribute_schema_id,
    old_value,
    new_value,
    timestamp
)
VALUES ($1, $2, $3, $4, $5, $6);
