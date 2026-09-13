package capability

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/enterpilot/gomodel/internal/storage"
	"github.com/enterpilot/gomodel/internal/storage/sqlx"
)

// Result holds the initialized capability confirmation subsystem.
type Result struct {
	Service *Service
	Store   Store
}

// Close releases the store. The service holds no owned resources of its own.
func (r *Result) Close() error {
	if r == nil || r.Store == nil {
		return nil
	}
	return r.Store.Close()
}

// New creates the capability confirmation subsystem over an existing storage
// connection and immediately replays persisted confirmations into the
// registry (the startup load that makes confirmations survive restarts).
func New(ctx context.Context, shared storage.Storage, registry ModelRegistry) (*Result, error) {
	if shared == nil {
		return nil, fmt.Errorf("shared storage is required")
	}
	if registry == nil {
		return nil, fmt.Errorf("model registry is required")
	}
	store, err := createStore(ctx, shared)
	if err != nil {
		return nil, err
	}
	service := NewService(store, registry)
	if err := service.Refresh(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	return &Result{Service: service, Store: store}, nil
}

func createStore(ctx context.Context, store storage.Storage) (Store, error) {
	return storage.ResolveSQLBackend[Store](
		ctx,
		store,
		func(db sqlx.DB) (Store, error) { return NewSQLStore(ctx, db) },
		func(db *mongo.Database) (Store, error) { return NewMongoDBStore(db) },
	)
}
