-- name: InsertDomain :exec
INSERT INTO Domains (domain_id, domain_name, domain_controller, highest_usn, last_processed_usn)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (domain_id) DO NOTHING;

-- name: UpdateDomainLastProcessedUSN :exec
UPDATE Domains SET last_processed_usn = $1 WHERE domain_id = $2;

-- name: UpdateDomainHighestUSN :exec
UPDATE Domains SET highest_usn = $1 WHERE domain_id = $2;
