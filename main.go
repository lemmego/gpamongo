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

// Provider implements gpa.Provider using MongoDB
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
func GetRepository[T any](p *Provider) gpa.Repository[T] {
	var zero T
	collectionName := getCollectionName(zero)
	collection := p.database.Collection(collectionName)
	return NewRepository[T](collection, p)
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