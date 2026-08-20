-- A generation is a named in-flight branch. The canonical tables
-- (application, application_config, charm, resource, application_resource)
-- hold the committed state ("main"/"master"); a branch records application
-- scoped deltas in the generation_application_* tables, applied only to units
-- that track it via generation_unit.

-- generation_state enumerates the lifecycle states of a branch.
CREATE TABLE generation_state (
    id INT PRIMARY KEY,
    name TEXT NOT NULL
);

INSERT INTO generation_state VALUES
(0, 'in-flight'),
(1, 'committed'),
(2, 'aborted');

-- generation is a branch. generation_id is a monotonic, human-facing sequence
-- number allocated from the sequence table (namespace: generation).
-- name is unique amongst in-flight branches; committed and aborted branches
-- free their name for reuse.
CREATE TABLE generation (
    uuid TEXT NOT NULL PRIMARY KEY,
    generation_id INT NOT NULL,
    name TEXT NOT NULL,
    state_id INT NOT NULL,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    completed_by TEXT,
    completed_at DATETIME,
    CONSTRAINT fk_generation_state
    FOREIGN KEY (state_id)
    REFERENCES generation_state (id)
);

CREATE UNIQUE INDEX idx_generation_generation_id
ON generation (generation_id);

CREATE UNIQUE INDEX idx_generation_name
ON generation (name) WHERE state_id = 0;

-- generation_unit records which units track a branch. A unit tracks at most
-- one branch at a time; the unit_uuid index supports resolving the branch a
-- unit tracks in O(1).
CREATE TABLE generation_unit (
    generation_uuid TEXT NOT NULL,
    unit_uuid TEXT NOT NULL,
    CONSTRAINT fk_generation_unit_generation
    FOREIGN KEY (generation_uuid)
    REFERENCES generation (uuid),
    CONSTRAINT fk_generation_unit_unit
    FOREIGN KEY (unit_uuid)
    REFERENCES unit (uuid),
    PRIMARY KEY (generation_uuid, unit_uuid)
);

CREATE UNIQUE INDEX idx_generation_unit_unit
ON generation_unit (unit_uuid);

-- generation_application_charm overrides the charm a tracking unit runs for an
-- application under a branch. The override is a reference to an existing charm.
CREATE TABLE generation_application_charm (
    generation_uuid TEXT NOT NULL,
    application_uuid TEXT NOT NULL,
    charm_uuid TEXT NOT NULL,
    CONSTRAINT fk_generation_application_charm_generation
    FOREIGN KEY (generation_uuid)
    REFERENCES generation (uuid),
    CONSTRAINT fk_generation_application_charm_application
    FOREIGN KEY (application_uuid)
    REFERENCES application (uuid),
    CONSTRAINT fk_generation_application_charm_charm
    FOREIGN KEY (charm_uuid)
    REFERENCES charm (uuid),
    PRIMARY KEY (generation_uuid, application_uuid)
);

CREATE INDEX idx_generation_application_charm_application
ON generation_application_charm (application_uuid);

-- generation_application_config holds config deltas for an application under a
-- branch. Absence of a row means "inherit master". A row with a NULL value is
-- an explicit unset (tombstone): revert to the charm default, overriding any
-- user-set value on master.
CREATE TABLE generation_application_config (
    generation_uuid TEXT NOT NULL,
    application_uuid TEXT NOT NULL,
    "key" TEXT NOT NULL,
    type_id INT NOT NULL,
    value TEXT,
    CONSTRAINT fk_generation_application_config_generation
    FOREIGN KEY (generation_uuid)
    REFERENCES generation (uuid),
    CONSTRAINT fk_generation_application_config_application
    FOREIGN KEY (application_uuid)
    REFERENCES application (uuid),
    CONSTRAINT fk_generation_application_config_type
    FOREIGN KEY (type_id)
    REFERENCES charm_config_type (id),
    PRIMARY KEY (generation_uuid, application_uuid, "key")
);

CREATE INDEX idx_generation_application_config_application
ON generation_application_config (application_uuid);

-- generation_application_resource overrides the resource revision an
-- application uses under a branch, keyed by the resource name.
CREATE TABLE generation_application_resource (
    generation_uuid TEXT NOT NULL,
    application_uuid TEXT NOT NULL,
    charm_resource_name TEXT NOT NULL,
    resource_uuid TEXT NOT NULL,
    CONSTRAINT fk_generation_application_resource_generation
    FOREIGN KEY (generation_uuid)
    REFERENCES generation (uuid),
    CONSTRAINT fk_generation_application_resource_application
    FOREIGN KEY (application_uuid)
    REFERENCES application (uuid),
    CONSTRAINT fk_generation_application_resource_resource
    FOREIGN KEY (resource_uuid)
    REFERENCES resource (uuid),
    PRIMARY KEY (generation_uuid, application_uuid, charm_resource_name)
);

CREATE INDEX idx_generation_application_resource_application
ON generation_application_resource (application_uuid);

-- generation_commit is the immutable history of committed branches, kept
-- separate from in-flight generation rows. The deltas are frozen at commit
-- time in generation_commit_config.
CREATE TABLE generation_commit (
    uuid TEXT NOT NULL PRIMARY KEY,
    generation_uuid TEXT NOT NULL,
    generation_id INT NOT NULL,
    name TEXT NOT NULL,
    created_by TEXT NOT NULL,
    committed_by TEXT NOT NULL,
    committed_at DATETIME NOT NULL,
    CONSTRAINT fk_generation_commit_generation
    FOREIGN KEY (generation_uuid)
    REFERENCES generation (uuid)
);

CREATE UNIQUE INDEX idx_generation_commit_generation_id
ON generation_commit (generation_id);

-- generation_commit_config stores the config deltas frozen at commit time.
CREATE TABLE generation_commit_config (
    commit_uuid TEXT NOT NULL,
    application_uuid TEXT NOT NULL,
    "key" TEXT NOT NULL,
    type_id INT NOT NULL,
    value TEXT,
    CONSTRAINT fk_generation_commit_config_commit
    FOREIGN KEY (commit_uuid)
    REFERENCES generation_commit (uuid),
    CONSTRAINT fk_generation_commit_config_type
    FOREIGN KEY (type_id)
    REFERENCES charm_config_type (id),
    PRIMARY KEY (commit_uuid, application_uuid, "key")
);

CREATE INDEX idx_generation_commit_config_application
ON generation_commit_config (application_uuid);
