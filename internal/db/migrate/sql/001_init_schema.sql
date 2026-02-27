CREATE TABLE IF NOT EXISTS outlines (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS outline_items (
    id INTEGER PRIMARY KEY,
    outline_id INTEGER NOT NULL,
    parent_id INTEGER,
    position INTEGER NOT NULL,
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'todo',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (outline_id) REFERENCES outlines(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES outline_items(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_outline_items_outline_position
    ON outline_items(outline_id, position);
