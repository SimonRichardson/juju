CREATE TABLE operator_status (
    application_uuid TEXT NOT NULL PRIMARY KEY,
    status_id INT NOT NULL,
    message TEXT,
    data TEXT,
    updated_at DATETIME,
    CONSTRAINT fk_application_status_application
    FOREIGN KEY (application_uuid)
    REFERENCES application (uuid),
    CONSTRAINT fk_workload_status_value_status
    FOREIGN KEY (status_id)
    REFERENCES workload_status_value (id)
);
