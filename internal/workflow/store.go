package workflow

import (
	"context"
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

type Item struct {
	UUID      string `json:"uuid"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Body      string `json:"body"`
	Children  []Item `json:"children"`
}

type itemRecord struct {
	UUID       string
	ParentUUID sql.NullString
	CreatedAt  string
	UpdatedAt  string
	Body       string
}

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("workflow store: nil database")
	}

	return &Store{db: db}, nil
}

func (s *Store) ListTree(ctx context.Context) ([]Item, error) {
	const query = `
SELECT uuid, parent_uuid, created_at, updated_at, body
FROM workflow_items
ORDER BY COALESCE(parent_uuid, ''), child_order, created_at, uuid
`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list workflow items: %w", err)
	}
	defer rows.Close()

	childrenByParent := make(map[string][]itemRecord)
	for rows.Next() {
		var record itemRecord
		if err := rows.Scan(&record.UUID, &record.ParentUUID, &record.CreatedAt, &record.UpdatedAt, &record.Body); err != nil {
			return nil, fmt.Errorf("scan workflow item: %w", err)
		}

		parentKey := ""
		if record.ParentUUID.Valid {
			parentKey = record.ParentUUID.String
		}

		childrenByParent[parentKey] = append(childrenByParent[parentKey], record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow items: %w", err)
	}

	return buildTree("", childrenByParent), nil
}

func buildTree(parent string, childrenByParent map[string][]itemRecord) []Item {
	records := childrenByParent[parent]
	items := make([]Item, 0, len(records))

	for _, record := range records {
		item := Item{
			UUID:      record.UUID,
			CreatedAt: record.CreatedAt,
			UpdatedAt: record.UpdatedAt,
			Body:      record.Body,
			Children:  buildTree(record.UUID, childrenByParent),
		}
		items = append(items, item)
	}

	return items
}
