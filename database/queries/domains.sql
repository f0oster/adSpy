-- name: InsertDomain :exec
INSERT INTO Domains (domain_id, domain_name, domain_controller, highest_usn, current_usn)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (domain_id) DO NOTHING;
