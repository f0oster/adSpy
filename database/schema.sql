CREATE TABLE Domains (
    domain_id UUID PRIMARY KEY,
    domain_name VARCHAR(255) NOT NULL,
    domain_controller VARCHAR NOT NULL,
    last_processed_usn BIGINT,
    highest_usn BIGINT
);

CREATE TABLE Objects (
    object_id UUID PRIMARY KEY,
    object_type VARCHAR(255) NOT NULL,
    distinguishedName TEXT NOT NULL,
    last_processed_usn BIGINT,
    domain_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE TABLE ObjectVersions (
    object_id UUID NOT NULL,
    usn_changed BIGINT NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    attributes_snapshot JSONB NOT NULL,
    modified_by VARCHAR(255),
    PRIMARY KEY (object_id, usn_changed)
);

CREATE TABLE AttributeChanges (
    object_id UUID NOT NULL,
    usn_changed BIGINT NOT NULL,
    attribute_schema_id UUID NOT NULL,
    old_value JSONB,
    new_value JSONB,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (object_id, usn_changed, attribute_schema_id)
);

-- Attribute Schema Registry
CREATE TABLE AttributeSchemas (
    object_guid UUID PRIMARY KEY,
    domain_id UUID NOT NULL,
    ldap_display_name VARCHAR(255) NOT NULL,
    attribute_name VARCHAR(255) NOT NULL,
    attribute_id VARCHAR(255) NOT NULL,
    attribute_syntax VARCHAR(255) NOT NULL,
    om_syntax VARCHAR(50) NOT NULL,
    syntax_name VARCHAR(255),
    is_single_valued BOOLEAN NOT NULL DEFAULT false
);


-- Foreign key constraints
ALTER TABLE Objects
ADD CONSTRAINT fk_objects_domain_id FOREIGN KEY (domain_id) REFERENCES Domains(domain_id);

ALTER TABLE ObjectVersions
ADD CONSTRAINT fk_object_versions_object_id FOREIGN KEY (object_id) REFERENCES Objects(object_id);

ALTER TABLE AttributeChanges
ADD CONSTRAINT fk_attribute_changes_version FOREIGN KEY (object_id, usn_changed) REFERENCES ObjectVersions(object_id, usn_changed),
ADD CONSTRAINT fk_attribute_changes_schema FOREIGN KEY (attribute_schema_id) REFERENCES AttributeSchemas(object_guid);

ALTER TABLE AttributeSchemas
ADD CONSTRAINT fk_attribute_schemas_domain_id FOREIGN KEY (domain_id) REFERENCES Domains(domain_id);

-- Indexes
CREATE INDEX idx_objects_domain_id ON Objects(domain_id);
CREATE INDEX idx_objects_object_type ON Objects(object_type);
CREATE INDEX idx_objects_dn ON Objects(distinguishedName);
CREATE INDEX idx_object_versions_object_id ON ObjectVersions(object_id);
CREATE INDEX idx_object_versions_timestamp ON ObjectVersions(timestamp);
CREATE INDEX idx_attribute_changes_object_id ON AttributeChanges(object_id);
CREATE INDEX idx_attribute_changes_usn ON AttributeChanges(usn_changed);
CREATE INDEX idx_attribute_changes_schema_id ON AttributeChanges(attribute_schema_id);
CREATE UNIQUE INDEX idx_attribute_schemas_domain_ldap ON AttributeSchemas(domain_id, ldap_display_name);
