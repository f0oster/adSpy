CREATE TABLE Domains (
    domain_id UUID PRIMARY KEY,
    domain_name VARCHAR(255) NOT NULL,
    schema_metadata JSONB,
    domain_controller VARCHAR NOT NULL,
    current_usn BIGINT,
    highest_usn BIGINT
);

CREATE TABLE Objects (
    object_id UUID PRIMARY KEY,
    object_type VARCHAR(255) NOT NULL,
    distinguishedName VARCHAR(255),
    current_version UUID,
    domain_id UUID,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE TABLE ObjectVersions (
    version_id UUID PRIMARY KEY NOT NULL,
    object_id UUID NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    attributes_snapshot JSONB NOT NULL,
    modified_by VARCHAR(255)
);

CREATE TABLE AttributeChanges (
    change_id UUID PRIMARY KEY,
    object_id UUID NOT NULL,
    attribute_name VARCHAR(255) NOT NULL,
    old_value JSONB,
    new_value JSONB,
    version_id UUID NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Foreign key constraints
ALTER TABLE Objects
ADD CONSTRAINT fk_objects_current_version FOREIGN KEY (current_version) REFERENCES ObjectVersions(version_id),
ADD CONSTRAINT fk_objects_domain_id FOREIGN KEY (domain_id) REFERENCES Domains(domain_id);

ALTER TABLE ObjectVersions
ADD CONSTRAINT fk_object_versions_object_id FOREIGN KEY (object_id) REFERENCES Objects(object_id);

ALTER TABLE AttributeChanges
ADD CONSTRAINT fk_attribute_changes_object_id FOREIGN KEY (object_id) REFERENCES Objects(object_id),
ADD CONSTRAINT fk_attribute_changes_version_id FOREIGN KEY (version_id) REFERENCES ObjectVersions(version_id);

-- Indexes
CREATE INDEX idx_objects_domain_id ON Objects(domain_id);
CREATE INDEX idx_objects_object_type ON Objects(object_type);
CREATE INDEX idx_object_versions_object_id ON ObjectVersions(object_id);
CREATE INDEX idx_object_versions_timestamp ON ObjectVersions(timestamp);
CREATE INDEX idx_attribute_changes_object_id ON AttributeChanges(object_id);
CREATE INDEX idx_attribute_changes_version_id ON AttributeChanges(version_id);
CREATE INDEX idx_attribute_changes_attribute_name ON AttributeChanges(attribute_name);
