-- 1. Domains Table
CREATE TABLE Domains (
    domain_id UUID PRIMARY KEY,
    domain_name VARCHAR(255) NOT NULL,
    schema_metadata JSONB,
    domain_controller VARCHAR NOT NULL,
    last_processed_usn BIGINT,
    highest_usn BIGINT
);

-- 2. Objects Table
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

-- 3. ObjectVersions Table
CREATE TABLE ObjectVersions (
    version_id UUID PRIMARY KEY,
    object_id UUID NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    attributes_snapshot JSONB NOT NULL,
    modified_by VARCHAR(255)
);

-- 4. AttributeChanges Table
CREATE TABLE AttributeChanges (
    change_id UUID PRIMARY KEY,
    object_id UUID NOT NULL,
    attribute_name VARCHAR(255) NOT NULL,
    old_value JSONB,
    new_value JSONB,
    version_id UUID NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add foreign key references
ALTER TABLE Objects
ADD CONSTRAINT fk_objects_current_version FOREIGN KEY (current_version) REFERENCES ObjectVersions(version_id),
ADD CONSTRAINT fk_objects_domain_id FOREIGN KEY (domain_id) REFERENCES Domains(domain_id);

ALTER TABLE ObjectVersions
ADD CONSTRAINT fk_object_versions_object_id FOREIGN KEY (object_id) REFERENCES Objects(object_id);

ALTER TABLE AttributeChanges
ADD CONSTRAINT fk_attribute_changes_object_id FOREIGN KEY (object_id) REFERENCES Objects(object_id),
ADD CONSTRAINT fk_attribute_changes_version_id FOREIGN KEY (version_id) REFERENCES ObjectVersions(version_id);