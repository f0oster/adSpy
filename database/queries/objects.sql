-- name: UpsertObject :one
INSERT INTO Objects (object_id, object_type, distinguishedName, domain_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (object_id)
DO UPDATE SET
    updated_at = NOW(),
    distinguishedName = EXCLUDED.distinguishedName,
    object_type = EXCLUDED.object_type
RETURNING last_processed_usn;

-- name: UpdateLastProcessedUSN :exec
UPDATE Objects
SET last_processed_usn = $1
WHERE object_id = $2;
