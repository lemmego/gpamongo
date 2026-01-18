// Package gpamongo provides a MongoDB adapter for the Go Persistence API (GPA)
package gpamongo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lemmego/gpa"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// =====================================
// Provider Implementation
// =====================================

// Provider implements gpa.Provider and gpa.DocumentProvider using MongoDB
type Provider struct {
	client   *mongo.Client
	database *mongo.Database
	config   gpa.Config
}

// NewProvider creates a new MongoDB provider instance
func NewProvider(config gpa.Config) (*Provider, error) {
	provider := &Provider{config: config}

	// Build connection string
	connectionURI := buildConnectionURI(config)

	// Create client options
	clientOpts := options.Client().ApplyURI(connectionURI)

	// Apply additional options
	if opts, ok := config.Options["mongo"]; ok {
		if mongoOpts, ok := opts.(map[string]interface{}); ok {
			applyClientOptions(clientOpts, mongoOpts)
		}
	}

	// Create MongoDB client
	client, err := mongo.Connect(context.Background(), clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	provider.client = client
	provider.database = client.Database(config.Database)

	return provider, nil
}

// Configure applies configuration to the provider
func (p *Provider) Configure(config gpa.Config) error {
	p.config = config
	return nil
}

// Health checks if the MongoDB connection is healthy
func (p *Provider) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.client.Ping(ctx, readpref.Primary())
}

// Close closes the MongoDB connection
func (p *Provider) Close() error {
	return p.client.Disconnect(context.Background())
}

// SupportedFeatures returns the features supported by MongoDB
func (p *Provider) SupportedFeatures() []gpa.Feature {
	return []gpa.Feature{
		gpa.FeatureTransactions,
		gpa.FeatureIndexes,
		gpa.FeatureFullText,
		gpa.FeatureGeospatial,
		gpa.FeatureAggregation,
		gpa.FeatureSharding,
		gpa.FeatureReplication,
	}
}

// ProviderInfo returns information about the MongoDB provider
func (p *Provider) ProviderInfo() gpa.ProviderInfo {
	return gpa.ProviderInfo{
		Name:         "MongoDB",
		Version:      "1.0.0",
		DatabaseType: gpa.DatabaseTypeDocument,
		Features:     p.SupportedFeatures(),
	}
}

// GetRepository returns a type-safe repository for any entity type T
// This enables the unified provider API: userRepo := gpamongo.GetRepository[User](provider)
func GetRepository[T any](p *Provider) gpa.DocumentRepository[T] {
	var zero T
	collectionName := getCollectionName(zero)
	collection := p.database.Collection(collectionName)
	return NewRepository[T](collection, p)
}

// =====================================
// DocumentProvider Implementation
// =====================================

// Database returns the underlying MongoDB database instance
func (p *Provider) Database() interface{} {
	return p.database
}

// Collection returns a collection instance
func (p *Provider) Collection(name string) interface{} {
	return p.database.Collection(name)
}

// CreateIndex creates an index on a collection
func (p *Provider) CreateIndex(ctx context.Context, collection string, keys interface{}, indexOptions *gpa.IndexOptions) error {
	coll := p.database.Collection(collection)
	
	indexModel := mongo.IndexModel{
		Keys: keys,
	}
	
	if indexOptions != nil {
		mongoOpts := &options.IndexOptions{}
		if indexOptions.Unique {
			mongoOpts.SetUnique(true)
		}
		if indexOptions.Sparse {
			mongoOpts.SetSparse(true)
		}
		if indexOptions.Background {
			mongoOpts.SetBackground(true)
		}
		if indexOptions.Name != "" {
			mongoOpts.SetName(indexOptions.Name)
		}
		if indexOptions.TTL > 0 {
			mongoOpts.SetExpireAfterSeconds(int32(indexOptions.TTL.Seconds()))
		}
		indexModel.Options = mongoOpts
	}
	
	_, err := coll.Indexes().CreateOne(ctx, indexModel)
	return err
}

// DropIndex drops an index from a collection
func (p *Provider) DropIndex(ctx context.Context, collection string, name string) error {
	coll := p.database.Collection(collection)
	_, err := coll.Indexes().DropOne(ctx, name)
	return err
}

// ListIndexes lists all indexes for a collection
func (p *Provider) ListIndexes(ctx context.Context, collection string) ([]gpa.IndexInfo, error) {
	coll := p.database.Collection(collection)
	cursor, err := coll.Indexes().List(ctx)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	
	var indexes []gpa.IndexInfo
	for cursor.Next(ctx) {
		var indexDoc map[string]interface{}
		if err := cursor.Decode(&indexDoc); err != nil {
			continue
		}
		
		indexInfo := gpa.IndexInfo{
			Name:     indexDoc["name"].(string),
			Fields:   []string{}, // Convert keys to field names
			IsUnique: false,
			Type:     gpa.IndexTypeStandard,
		}
		
		// Convert MongoDB key document to field names
		if keyDoc, ok := indexDoc["key"].(map[string]interface{}); ok {
			for field := range keyDoc {
				indexInfo.Fields = append(indexInfo.Fields, field)
			}
		}
		
		if unique, ok := indexDoc["unique"]; ok {
			indexInfo.IsUnique = unique.(bool)
		}
		
		indexes = append(indexes, indexInfo)
	}
	
	return indexes, cursor.Err()
}

// Aggregate runs an aggregation pipeline
func (p *Provider) Aggregate(ctx context.Context, collection string, pipeline interface{}) (interface{}, error) {
	coll := p.database.Collection(collection)
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	
	var results []interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	
	return results, nil
}

// Watch starts a change stream
func (p *Provider) Watch(ctx context.Context, collection string, pipeline interface{}) (interface{}, error) {
	coll := p.database.Collection(collection)
	return coll.Watch(ctx, pipeline)
}

// getCollectionName returns the collection name for a type
func getCollectionName(entity interface{}) string {
	// Simple implementation - use the type name
	// In production, you might want to use struct tags or other mechanisms
	typeName := fmt.Sprintf("%T", entity)
	// Remove package prefix if present
	if idx := strings.LastIndex(typeName, "."); idx > 0 {
		typeName = typeName[idx+1:]
	}
	return strings.ToLower(typeName) + "s"
}

// =====================================
// Helper Functions
// =====================================

func buildConnectionURI(config gpa.Config) string {
	if config.ConnectionURL != "" {
		return config.ConnectionURL
	}

	host := config.Host
	if host == "" {
		host = "localhost"
	}

	port := config.Port
	if port == 0 {
		port = 27017
	}

	if config.Username != "" && config.Password != "" {
		return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s", 
			config.Username, config.Password, host, port, config.Database)
	}

	return fmt.Sprintf("mongodb://%s:%d/%s", host, port, config.Database)
}

func applyClientOptions(clientOpts *options.ClientOptions, mongoOpts map[string]interface{}) {
	if maxPoolSize, ok := mongoOpts["max_pool_size"]; ok {
		if size, ok := maxPoolSize.(uint64); ok {
			clientOpts.SetMaxPoolSize(size)
		}
	}
	if minPoolSize, ok := mongoOpts["min_pool_size"]; ok {
		if size, ok := minPoolSize.(uint64); ok {
			clientOpts.SetMinPoolSize(size)
		}
	}
	if maxIdleTime, ok := mongoOpts["max_idle_time"]; ok {
		if duration, ok := maxIdleTime.(time.Duration); ok {
			clientOpts.SetMaxConnIdleTime(duration)
		}
	}
}