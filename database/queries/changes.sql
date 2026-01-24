-- name: InsertAttributeChange :exec
INSERT INTO AttributeChanges (
    change_id,
    object_id,
    attribute_name,
    old_value,
    new_value,
    version_id,
    timestamp
)
VALUES ($1, $2, $3, $4, $5, $6, $7);
