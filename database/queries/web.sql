-- name: ListObjectsForWeb :many
SELECT object_id, object_type, distinguishedName, updated_at, deleted_at
FROM Objects
WHERE deleted_at IS NULL
  AND ($1::text = '' OR object_type = $1)
  AND ($2::text = '' OR distinguishedName ILIKE '%' || $2 || '%')
ORDER BY updated_at DESC
LIMIT $3 OFFSET $4;

-- name: CountObjectsForWeb :one
SELECT COUNT(*) as total
FROM Objects
WHERE deleted_at IS NULL
  AND ($1::text = '' OR object_type = $1)
  AND ($2::text = '' OR distinguishedName ILIKE '%' || $2 || '%');

-- name: GetObjectByID :one
SELECT object_id, object_type, distinguishedName, updated_at, deleted_at
FROM Objects
WHERE object_id = $1;

-- name: GetObjectTimeline :many
SELECT usn_changed, timestamp, attributes_snapshot, modified_by
FROM ObjectVersions
WHERE object_id = $1
ORDER BY usn_changed DESC;

-- name: GetVersionChanges :many
SELECT ac.attribute_schema_id, s.ldap_display_name, ac.old_value, ac.new_value, ac.timestamp, s.is_single_valued
FROM AttributeChanges ac
JOIN AttributeSchemas s ON ac.attribute_schema_id = s.object_guid
WHERE ac.object_id = $1 AND ac.usn_changed = $2
ORDER BY s.ldap_display_name;

-- name: GetObjectTypes :many
SELECT DISTINCT object_type
FROM Objects
WHERE deleted_at IS NULL
ORDER BY object_type;
