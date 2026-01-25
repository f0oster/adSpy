-- name: InsertVersion :exec
INSERT INTO ObjectVersions (object_id, usn_changed, timestamp, attributes_snapshot, modified_by)
VALUES ($1, $2, $3, $4, $5);

-- name: GetPreviousSnapshot :one
SELECT attributes_snapshot
FROM ObjectVersions
WHERE object_id = $1 AND usn_changed = $2;
