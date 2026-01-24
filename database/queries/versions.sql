-- name: InsertVersion :exec
INSERT INTO ObjectVersions (version_id, object_id, timestamp, attributes_snapshot, modified_by)
VALUES ($1, $2, $3, $4, $5);

-- name: GetPreviousSnapshot :one
SELECT attributes_snapshot
FROM ObjectVersions
WHERE version_id = $1;
