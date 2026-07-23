-- namespace_list maintains a list of tracked dqlite namespaces for the
-- controller.
CREATE TABLE namespace_list (
    namespace TEXT NOT NULL PRIMARY KEY
);

-- dqlite_application describes the stable set of Dqlite applications used to
-- host model databases. Application zero is reserved for the controller
-- database and is therefore not represented here.
CREATE TABLE dqlite_application (
    id INT NOT NULL PRIMARY KEY,
    state TEXT NOT NULL DEFAULT 'ready',
    capacity INT NOT NULL,
    CONSTRAINT chk_dqlite_application_id CHECK (id > 0),
    CONSTRAINT chk_dqlite_application_state
        CHECK (state IN ('provisioning', 'ready', 'failed')),
    CONSTRAINT chk_dqlite_application_capacity CHECK (capacity > 0)
);

-- Bootstrap updates this capacity and adds further applications before
-- creating the controller model when configured.
INSERT INTO dqlite_application (id, state, capacity)
VALUES (1, 'ready', 20);
