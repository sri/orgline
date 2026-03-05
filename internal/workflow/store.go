package workflow

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
)

var ErrItemNotFound = errors.New("workflow item not found")

type Store struct {
	db *sql.DB
}

type Item struct {
	UUID       string `json:"uuid"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	Body       string `json:"body"`
	IsOpen     bool   `json:"is_open"`
	IsFavorite bool   `json:"is_favorite"`
	Children   []Item `json:"children"`
}

type itemRecord struct {
	UUID       string
	ParentUUID sql.NullString
	CreatedAt  string
	UpdatedAt  string
	Body       string
	IsOpen     int64
	IsFavorite int64
}

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("workflow store: nil database")
	}

	return &Store{db: db}, nil
}

func (s *Store) ListTree(ctx context.Context) ([]Item, error) {
	const query = `
SELECT uuid, parent_uuid, created_at, updated_at, body, is_open, is_favorite
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
		if err := rows.Scan(
			&record.UUID,
			&record.ParentUUID,
			&record.CreatedAt,
			&record.UpdatedAt,
			&record.Body,
			&record.IsOpen,
			&record.IsFavorite,
		); err != nil {
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
			UUID:       record.UUID,
			CreatedAt:  record.CreatedAt,
			UpdatedAt:  record.UpdatedAt,
			Body:       record.Body,
			IsOpen:     record.IsOpen != 0,
			IsFavorite: record.IsFavorite != 0,
			Children:   buildTree(record.UUID, childrenByParent),
		}
		items = append(items, item)
	}

	return items
}

func (s *Store) UpdateBody(ctx context.Context, uuid string, body string) error {
	const statement = `
UPDATE workflow_items
SET body = ?, updated_at = CURRENT_TIMESTAMP
WHERE uuid = ?
`

	result, err := s.db.ExecContext(ctx, statement, body, uuid)
	if err != nil {
		return fmt.Errorf("update workflow item body: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated rows: %w", err)
	}

	if rowsAffected == 0 {
		return ErrItemNotFound
	}

	return nil
}

func (s *Store) UpdateOpenState(ctx context.Context, uuid string, isOpen bool) error {
	const statement = `
UPDATE workflow_items
SET is_open = ?, updated_at = CURRENT_TIMESTAMP
WHERE uuid = ?
`

	var openValue int
	if isOpen {
		openValue = 1
	}

	result, err := s.db.ExecContext(ctx, statement, openValue, uuid)
	if err != nil {
		return fmt.Errorf("update workflow item open state: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated rows: %w", err)
	}

	if rowsAffected == 0 {
		return ErrItemNotFound
	}

	return nil
}

func (s *Store) UpdateFavoriteState(ctx context.Context, uuid string, isFavorite bool) error {
	const statement = `
UPDATE workflow_items
SET is_favorite = ?, updated_at = CURRENT_TIMESTAMP
WHERE uuid = ?
`

	var favoriteValue int
	if isFavorite {
		favoriteValue = 1
	}

	result, err := s.db.ExecContext(ctx, statement, favoriteValue, uuid)
	if err != nil {
		return fmt.Errorf("update workflow item favorite state: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated rows: %w", err)
	}

	if rowsAffected == 0 {
		return ErrItemNotFound
	}

	return nil
}

func (s *Store) DeleteItem(ctx context.Context, uuid string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete transaction: %w", err)
	}

	parent, order, err := getCurrentItemPosition(ctx, tx, uuid)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	result, err := tx.ExecContext(ctx, "DELETE FROM workflow_items WHERE uuid = ?", uuid)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete workflow item %q: %w", uuid, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("read deleted rows: %w", err)
	}
	if rowsAffected == 0 {
		_ = tx.Rollback()
		return ErrItemNotFound
	}

	if err := shiftAfterRemoval(ctx, tx, parent, order); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete transaction: %w", err)
	}

	return nil
}

func (s *Store) CreateAfterEnter(ctx context.Context, currentUUID string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin create-after-enter transaction: %w", err)
	}

	currentParent, currentOrder, err := getCurrentItemPosition(ctx, tx, currentUUID)
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}

	hasChildren, err := itemHasChildren(ctx, tx, currentUUID)
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}

	targetParent := currentParent
	targetOrder := currentOrder + 1

	if hasChildren {
		targetParent = sql.NullString{String: currentUUID, Valid: true}
		targetOrder = 1
	}

	if err := shiftChildrenOrder(ctx, tx, targetParent, targetOrder); err != nil {
		_ = tx.Rollback()
		return "", err
	}

	if hasChildren {
		if _, err := tx.ExecContext(
			ctx,
			"UPDATE workflow_items SET is_open = 1, updated_at = CURRENT_TIMESTAMP WHERE uuid = ?",
			currentUUID,
		); err != nil {
			_ = tx.Rollback()
			return "", fmt.Errorf("open parent item %q: %w", currentUUID, err)
		}
	}

	newUUID, err := generateUUIDv4()
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}

	if err := insertWorkflowItem(ctx, tx, newUUID, targetParent, targetOrder); err != nil {
		_ = tx.Rollback()
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit create-after-enter transaction: %w", err)
	}

	return newUUID, nil
}

func getCurrentItemPosition(ctx context.Context, tx *sql.Tx, uuid string) (sql.NullString, int64, error) {
	const query = `
SELECT parent_uuid, child_order
FROM workflow_items
WHERE uuid = ?
`

	var parent sql.NullString
	var order int64
	err := tx.QueryRowContext(ctx, query, uuid).Scan(&parent, &order)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.NullString{}, 0, ErrItemNotFound
	}
	if err != nil {
		return sql.NullString{}, 0, fmt.Errorf("read current workflow item %q: %w", uuid, err)
	}

	return parent, order, nil
}

func itemHasChildren(ctx context.Context, tx *sql.Tx, uuid string) (bool, error) {
	const query = `
SELECT EXISTS(
    SELECT 1
    FROM workflow_items
    WHERE parent_uuid = ?
)
`

	var hasChildren bool
	if err := tx.QueryRowContext(ctx, query, uuid).Scan(&hasChildren); err != nil {
		return false, fmt.Errorf("check children for item %q: %w", uuid, err)
	}

	return hasChildren, nil
}

func shiftChildrenOrder(ctx context.Context, tx *sql.Tx, parent sql.NullString, fromOrder int64) error {
	var (
		statement string
		args      []any
	)

	if parent.Valid {
		statement = `
UPDATE workflow_items
SET child_order = child_order + 1
WHERE parent_uuid = ? AND child_order >= ?
`
		args = []any{parent.String, fromOrder}
	} else {
		statement = `
UPDATE workflow_items
SET child_order = child_order + 1
WHERE parent_uuid IS NULL AND child_order >= ?
`
		args = []any{fromOrder}
	}

	if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
		return fmt.Errorf("shift sibling order from %d: %w", fromOrder, err)
	}

	return nil
}

func insertWorkflowItem(ctx context.Context, tx *sql.Tx, uuid string, parent sql.NullString, order int64) error {
	const statement = `
INSERT INTO workflow_items (uuid, parent_uuid, child_order, body, is_open)
VALUES (?, ?, ?, '', 1)
`

	var parentValue any
	if parent.Valid {
		parentValue = parent.String
	} else {
		parentValue = nil
	}

	if _, err := tx.ExecContext(ctx, statement, uuid, parentValue, order); err != nil {
		return fmt.Errorf("insert workflow item %q: %w", uuid, err)
	}

	return nil
}

func generateUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[0], b[1], b[2], b[3],
		b[4], b[5],
		b[6], b[7],
		b[8], b[9],
		b[10], b[11], b[12], b[13], b[14], b[15],
	), nil
}

func (s *Store) IndentItem(ctx context.Context, uuid string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin indent transaction: %w", err)
	}

	currentParent, currentOrder, err := getCurrentItemPosition(ctx, tx, uuid)
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}

	previousSiblingUUID, found, err := previousSiblingUUID(ctx, tx, currentParent, currentOrder)
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if !found {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit indent no-op transaction: %w", err)
		}
		return false, nil
	}

	if err := shiftAfterRemoval(ctx, tx, currentParent, currentOrder); err != nil {
		_ = tx.Rollback()
		return false, err
	}

	newOrder, err := maxChildOrder(ctx, tx, previousSiblingUUID)
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	newOrder++

	if _, err := tx.ExecContext(
		ctx,
		"UPDATE workflow_items SET parent_uuid = ?, child_order = ?, updated_at = CURRENT_TIMESTAMP WHERE uuid = ?",
		previousSiblingUUID,
		newOrder,
		uuid,
	); err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("indent item %q: %w", uuid, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		"UPDATE workflow_items SET is_open = 1, updated_at = CURRENT_TIMESTAMP WHERE uuid = ?",
		previousSiblingUUID,
	); err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("open previous sibling %q: %w", previousSiblingUUID, err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit indent transaction: %w", err)
	}

	return true, nil
}

func (s *Store) OutdentItem(ctx context.Context, uuid string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin outdent transaction: %w", err)
	}

	currentParent, currentOrder, err := getCurrentItemPosition(ctx, tx, uuid)
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if !currentParent.Valid {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit outdent no-op transaction: %w", err)
		}
		return false, nil
	}

	parentParent, parentOrder, err := getCurrentItemPosition(ctx, tx, currentParent.String)
	if err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("read parent %q: %w", currentParent.String, err)
	}

	if err := shiftAfterRemoval(ctx, tx, currentParent, currentOrder); err != nil {
		_ = tx.Rollback()
		return false, err
	}

	newOrder := parentOrder + 1
	if err := shiftAtInsertion(ctx, tx, parentParent, newOrder); err != nil {
		_ = tx.Rollback()
		return false, err
	}

	var parentValue any
	if parentParent.Valid {
		parentValue = parentParent.String
	} else {
		parentValue = nil
	}

	if _, err := tx.ExecContext(
		ctx,
		"UPDATE workflow_items SET parent_uuid = ?, child_order = ?, updated_at = CURRENT_TIMESTAMP WHERE uuid = ?",
		parentValue,
		newOrder,
		uuid,
	); err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("outdent item %q: %w", uuid, err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit outdent transaction: %w", err)
	}

	return true, nil
}

func previousSiblingUUID(
	ctx context.Context,
	tx *sql.Tx,
	parent sql.NullString,
	currentOrder int64,
) (string, bool, error) {
	var (
		query string
		args  []any
	)

	if parent.Valid {
		query = `
SELECT uuid
FROM workflow_items
WHERE parent_uuid = ? AND child_order = ?
`
		args = []any{parent.String, currentOrder - 1}
	} else {
		query = `
SELECT uuid
FROM workflow_items
WHERE parent_uuid IS NULL AND child_order = ?
`
		args = []any{currentOrder - 1}
	}

	var siblingUUID string
	err := tx.QueryRowContext(ctx, query, args...).Scan(&siblingUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read previous sibling: %w", err)
	}

	return siblingUUID, true, nil
}

func shiftAfterRemoval(ctx context.Context, tx *sql.Tx, parent sql.NullString, currentOrder int64) error {
	var (
		query string
		args  []any
	)

	if parent.Valid {
		query = `
UPDATE workflow_items
SET child_order = child_order - 1
WHERE parent_uuid = ? AND child_order > ?
`
		args = []any{parent.String, currentOrder}
	} else {
		query = `
UPDATE workflow_items
SET child_order = child_order - 1
WHERE parent_uuid IS NULL AND child_order > ?
`
		args = []any{currentOrder}
	}

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("shift siblings after removal at order %d: %w", currentOrder, err)
	}

	return nil
}

func shiftAtInsertion(ctx context.Context, tx *sql.Tx, parent sql.NullString, insertAt int64) error {
	var (
		query string
		args  []any
	)

	if parent.Valid {
		query = `
UPDATE workflow_items
SET child_order = child_order + 1
WHERE parent_uuid = ? AND child_order >= ?
`
		args = []any{parent.String, insertAt}
	} else {
		query = `
UPDATE workflow_items
SET child_order = child_order + 1
WHERE parent_uuid IS NULL AND child_order >= ?
`
		args = []any{insertAt}
	}

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("shift siblings for insertion at order %d: %w", insertAt, err)
	}

	return nil
}

func maxChildOrder(ctx context.Context, tx *sql.Tx, parentUUID string) (int64, error) {
	var maxOrder int64
	if err := tx.QueryRowContext(
		ctx,
		"SELECT COALESCE(MAX(child_order), 0) FROM workflow_items WHERE parent_uuid = ?",
		parentUUID,
	).Scan(&maxOrder); err != nil {
		return 0, fmt.Errorf("read max child order for %q: %w", parentUUID, err)
	}

	return maxOrder, nil
}
