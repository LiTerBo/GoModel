package capability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// mongoConfirmationDocument is the stored shape of one confirmation. The three
// key fields carry the same names the SQL schema uses, so an operator reading
// either backend sees the same vocabulary.
type mongoConfirmationDocument struct {
	Provider   string `bson:"provider"`
	Model      string `bson:"model"`
	Capability string `bson:"capability"`
	Source     string `bson:"source"`
	Value      bool   `bson:"value"`
	CreatedAt  int64  `bson:"created_at"`
}

// mongoConfirmationKey is the (provider, model, capability) primary key as a
// filter, which keeps the delete and upsert filters in one shape.
type mongoConfirmationKey struct {
	Provider   string `bson:"provider"`
	Model      string `bson:"model"`
	Capability string `bson:"capability"`
}

// MongoDBStore stores capability confirmations in MongoDB.
type MongoDBStore struct {
	collection *mongo.Collection
}

// NewMongoDBStore creates the capability_confirmations indexes if needed.
func NewMongoDBStore(database *mongo.Database) (*MongoDBStore, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	coll := database.Collection("capability_confirmations")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "provider", Value: 1}, {Key: "model", Value: 1}, {Key: "capability", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "model", Value: 1}}},
	}
	if _, err := coll.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("create capability_confirmations indexes: %w", err)
	}
	return &MongoDBStore{collection: coll}, nil
}

func (s *MongoDBStore) Upsert(ctx context.Context, c Confirmation) error {
	confirmation, err := normalizeConfirmation(c)
	if err != nil {
		return err
	}
	doc := mongoConfirmationDocument{
		Provider:   confirmation.Provider,
		Model:      confirmation.Model,
		Capability: confirmation.Capability,
		Source:     confirmation.Source,
		Value:      confirmation.Value,
		CreatedAt:  confirmation.CreatedAt,
	}
	filter := mongoConfirmationKey{
		Provider:   confirmation.Provider,
		Model:      confirmation.Model,
		Capability: confirmation.Capability,
	}
	if _, err := s.collection.ReplaceOne(ctx, filter, doc, options.Replace().SetUpsert(true)); err != nil {
		return fmt.Errorf("upsert capability confirmation: %w", err)
	}
	return nil
}

func (s *MongoDBStore) List(ctx context.Context) ([]Confirmation, error) {
	cursor, err := s.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{
		{Key: "provider", Value: 1},
		{Key: "model", Value: 1},
		{Key: "capability", Value: 1},
	}))
	if err != nil {
		return nil, fmt.Errorf("list capability confirmations: %w", err)
	}
	defer cursor.Close(ctx)

	result := make([]Confirmation, 0)
	for cursor.Next(ctx) {
		var doc mongoConfirmationDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode capability confirmation: %w", err)
		}
		result = append(result, Confirmation{
			Provider:   doc.Provider,
			Model:      doc.Model,
			Capability: doc.Capability,
			Source:     doc.Source,
			Value:      doc.Value,
			CreatedAt:  doc.CreatedAt,
		})
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate capability confirmations: %w", err)
	}
	return result, nil
}

func (s *MongoDBStore) Delete(ctx context.Context, provider, model, capability string) error {
	filter := mongoConfirmationKey{
		Provider:   strings.TrimSpace(provider),
		Model:      strings.TrimSpace(model),
		Capability: strings.TrimSpace(capability),
	}
	result, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("delete capability confirmation: %w", err)
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MongoDBStore) Close() error {
	return nil
}
