-- name: UpsertAttributeSchema :exec
INSERT INTO AttributeSchemas (
    object_guid, domain_id, ldap_display_name, attribute_name, attribute_id,
    attribute_syntax, om_syntax, syntax_name, is_single_valued
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (object_guid)
DO UPDATE SET
    domain_id = EXCLUDED.domain_id,
    ldap_display_name = EXCLUDED.ldap_display_name,
    attribute_name = EXCLUDED.attribute_name,
    attribute_id = EXCLUDED.attribute_id,
    attribute_syntax = EXCLUDED.attribute_syntax,
    om_syntax = EXCLUDED.om_syntax,
    syntax_name = EXCLUDED.syntax_name,
    is_single_valued = EXCLUDED.is_single_valued;

-- name: GetAttributeSchemaByLDAPName :one
SELECT object_guid
FROM AttributeSchemas
WHERE domain_id = $1 AND ldap_display_name = $2;
