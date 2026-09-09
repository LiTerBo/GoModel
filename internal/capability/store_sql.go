package capability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enterpilot/gomodel/internal/storage/sqlx"
)

// SQLStore stores capability confirmations in a SQL database.
type SQLStore struct {
	db sqlx.DB
}

var sqlSchema = []string{
	`CREATE TABLE IF NOT EXISTS capability_confirmations (
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		capability TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT 'test',
		value ` + sqlx.TypeInt64 + ` NOT NULL DEFAULT 1,
		created_at ` + sqlx.TypeInt64 + ` NOT NULL,
		PRIMARY KEY (provider, model, capability)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_capability_confirmations_model ON capability_confirmations(model)`,
}

// NewSQLStore creates the capability_confirmations table and indexes if needed.
func NewSQLStore(ctx context.Context, db sqlx.DB) (*SQLStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if err := db.Schema(ctx, sqlSchema...); err != nil {
		return nil, fmt.Errorf("failed to create capability_confirmations table: %w", err)
	}
	return &SQLStore{db: db}, nil
}

// normalizeConfirmation validates one confirmation and stamps CreatedAt when
// the caller left it zero. Provider/model/capability are the primary key, so
// they must be present; source defaults to test.
func normalizeConfirmation(c Confirmation) (Confirmation, error) {
	c.Provider = strings.TrimSpace(c.Provider)
	c.Model = strings.TrimSpace(c.Model)
	c.Capability = strings.TrimSpace(c.Capability)
	c.Source = strings.TrimSpace(c.Source)
	if c.Provider == "" || c.Model == "" || c.Capability == "" {
		return Confirmation{}, fmt.Errorf("provider, model and capability are required")
	}
	switch c.Source {
	case "":
		c.Source = SourceTest
	case SourceTest, SourceObserved:
	default:
		return Confirmation{}, fmt.Errorf("unknown source %q (valid: %s, %s)", c.Source, SourceTest, SourceObserved)
	}
	if c.CreatedAt == 0 {
		c.CreatedAt = time.Now().UTC().Unix()
	}
	return c, nil
}

func (s *SQLStore) Upsert(ctx context.Context, c Confirmation) error {
	c, err := normalizeConfirmation(c)
	if err != nil {
		return err
	}
	value := int64(0)
	if c.Value {
		value = 1
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO capability_confirmations (
			provider, model, capability, source, value, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, model, capability) DO UPDATE SET
			source = excluded.source,
			value = excluded.value,
			created_at = excluded.created_at
	`,
		c.Provider,
		c.Model,
		c.Capability,
		c.Source,
		value,
		c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert capability confirmation: %w", err)
	}
	return nil
}

func (s *SQLStore) List(ctx context.Context) ([]Confirmation, error) {
	rows, err := s.db.Query(ctx, `
		SELECT provider, model, capability, source, value, created_at
		FROM capability_confirmations
		ORDER BY provider ASC, model ASC, capability ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list capability confirmations: %w", err)
	}
	defer rows.Close()

	result := make([]Confirmation, 0)
	for rows.Next() {
		var c Confirmation
		var value int64
		if err := rows.Scan(
			&c.Provider,
			&c.Model,
			&c.Capability,
			&c.Source,
			&value,
			&c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan capability confirmation: %w", err)
		}
		c.Value = value != 0
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate capability confirmations: %w", err)
	}
	return result, nil
}

func (s *SQLStore) Delete(ctx context.Context, provider, model, capability string) error {
	affected, err := s.db.Exec(ctx,
		`DELETE FROM capability_confirmations WHERE provider = ? AND model = ? AND capability = ?`,
		strings.TrimSpace(provider), strings.TrimSpace(model), strings.TrimSpace(capability))
	if err != nil {
		return fmt.Errorf("delete capability confirmation: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) Close() error {
	return nil
}
