CREATE TABLE IF NOT EXISTS workflow_items (
    uuid TEXT PRIMARY KEY,
    parent_uuid TEXT,
    child_order INTEGER NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_uuid) REFERENCES workflow_items(uuid) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workflow_items_parent_order
    ON workflow_items(parent_uuid, child_order);
