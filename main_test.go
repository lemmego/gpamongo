package gpamongo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lemmego/gpa"
)

func getTestConfig() gpa.Config {
	// Check for MongoDB connection string in environment
	mongoURL := os.Getenv("MONGODB_TEST_URL")
	if mongoURL == "" {
		mongoURL = "mongodb://localhost:27017"
	}

	return gpa.Config{
		Driver:        "mongodb",
		ConnectionURL: mongoURL,
		Database:      "gpa_test",
	}
}

func skipIfNoMongo(t *testing.T) {
	config := getTestConfig()
	provider, err := NewProvider(config)
	if err != nil {
		t.Skipf("Skipping MongoDB tests: %v", err)
	}
	provider.Close()
}

func TestNewProvider(t *testing.T) {
	skipIfNoMongo(t)

	config := getTestConfig()
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	if provider.config.Database != "gpa_test" {
		t.Errorf("Expected database 'gpa_test', got '%s'", provider.config.Database)
	}
}

func TestProviderHealth(t *testing.T) {
	skipIfNoMongo(t)

	config := getTestConfig()
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	err = provider.Health()
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestProviderInfo(t *testing.T) {
	skipIfNoMongo(t)

	config := getTestConfig()
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	info := provider.ProviderInfo()
	if info.Name != "MongoDB" {
		t.Errorf("Expected name 'MongoDB', got '%s'", info.Name)
	}
	if info.DatabaseType != gpa.DatabaseTypeDocument {
		t.Errorf("Expected document database type, got %s", info.DatabaseType)
	}
	if len(info.Features) == 0 {
		t.Error("Expected features to be populated")
	}
}

func TestSupportedFeatures(t *testing.T) {
	skipIfNoMongo(t)

	config := getTestConfig()
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	features := provider.SupportedFeatures()
	expectedFeatures := []gpa.Feature{
		gpa.FeatureTransactions,
		gpa.FeatureIndexes,
		gpa.FeatureFullText,
		gpa.FeatureGeospatial,
		gpa.FeatureAggregation,
		gpa.FeatureSharding,
		gpa.FeatureReplication,
	}

	if len(features) != len(expectedFeatures) {
		t.Errorf("Expected %d features, got %d", len(expectedFeatures), len(features))
	}

	for _, expected := range expectedFeatures {
		found := false
		for _, feature := range features {
			if feature == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected feature '%s' not found", expected)
		}
	}
}

func TestUnifiedProviderAPI(t *testing.T) {
	skipIfNoMongo(t)

	type TestDoc struct {
		ID   string `bson:"_id,omitempty"`
		Name string `bson:"name"`
		Age  int    `bson:"age"`
	}

	config := getTestConfig()
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	// Test getting repository using unified API
	repo := GetRepository[TestDoc](provider)
	if repo == nil {
		t.Fatal("Expected repository to be created")
	}

	// Test provider methods
	err = provider.Health()
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}

	info := provider.ProviderInfo()
	if info.Name != "MongoDB" {
		t.Errorf("Expected name 'MongoDB', got '%s'", info.Name)
	}
}

func TestProviderConfigure(t *testing.T) {
	skipIfNoMongo(t)

	config := getTestConfig()
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	newConfig := gpa.Config{
		Driver:   "mongodb",
		Database: "new_test_db",
	}

	err = provider.Configure(newConfig)
	if err != nil {
		t.Errorf("Failed to configure provider: %v", err)
	}

	if provider.config.Database != "new_test_db" {
		t.Errorf("Expected database 'new_test_db', got '%s'", provider.config.Database)
	}
}

func TestBuildConnectionURI(t *testing.T) {
	// Test with connection URL
	config := gpa.Config{
		ConnectionURL: "mongodb://user:pass@localhost:27017/testdb",
	}
	uri := buildConnectionURI(config)
	if uri != config.ConnectionURL {
		t.Errorf("Expected connection URL to be used, got '%s'", uri)
	}

	// Test with individual parameters
	config = gpa.Config{
		Host:     "localhost",
		Port:     27017,
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}
	uri = buildConnectionURI(config)
	expected := "mongodb://user:pass@localhost:27017/testdb"
	if uri != expected {
		t.Errorf("Expected URI '%s', got '%s'", expected, uri)
	}

	// Test without credentials
	config = gpa.Config{
		Host:     "localhost",
		Port:     27017,
		Database: "testdb",
	}
	uri = buildConnectionURI(config)
	expected = "mongodb://localhost:27017/testdb"
	if uri != expected {
		t.Errorf("Expected URI '%s', got '%s'", expected, uri)
	}

	// Test with defaults
	config = gpa.Config{
		Database: "testdb",
	}
	uri = buildConnectionURI(config)
	expected = "mongodb://localhost:27017/testdb"
	if uri != expected {
		t.Errorf("Expected URI '%s', got '%s'", expected, uri)
	}
}

func TestGetCollectionName(t *testing.T) {
	type User struct {
		ID   string `bson:"_id"`
		Name string `bson:"name"`
	}

	user := User{}
	collectionName := getCollectionName(user)
	expected := "users"
	if collectionName != expected {
		t.Errorf("Expected collection name '%s', got '%s'", expected, collectionName)
	}
}

func TestApplyClientOptions(t *testing.T) {
	config := gpa.Config{
		Options: map[string]interface{}{
			"mongo": map[string]interface{}{
				"max_pool_size": uint64(50),
				"min_pool_size": uint64(5),
				"max_idle_time": time.Minute * 10,
			},
		},
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider with custom options: %v", err)
	}
	defer provider.Close()

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}
}

func TestProviderWithoutMongoDB(t *testing.T) {
	// Test with invalid connection
	config := gpa.Config{
		Driver:        "mongodb",
		ConnectionURL: "mongodb://invalid:27017",
		Database:      "test",
	}

	// Create a context with short timeout to fail quickly
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	provider, err := NewProvider(config)
	if err != nil {
		// This is expected if MongoDB is not available
		t.Logf("MongoDB not available: %v", err)
		return
	}
	defer provider.Close()

	// Try to ping with timeout context
	err = provider.client.Ping(ctx, nil)
	if err != nil {
		t.Logf("MongoDB ping failed: %v", err)
	}
}

func TestInvalidClientOptions(t *testing.T) {
	skipIfNoMongo(t)

	config := gpa.Config{
		Driver:        "mongodb",
		ConnectionURL: "mongodb://localhost:27017",
		Database:      "test",
		Options: map[string]interface{}{
			"mongo": map[string]interface{}{
				"max_pool_size": "invalid", // should be uint64
				"min_pool_size": -1,        // invalid value
				"max_idle_time": "invalid", // should be duration
			},
		},
	}

	// Should still work, just ignore invalid options
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider with invalid options: %v", err)
	}
	defer provider.Close()
}

func TestContextTimeout(t *testing.T) {
	skipIfNoMongo(t)

	config := getTestConfig()
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	// Create a context with very short timeout
	_, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	// This should work since the connection is already established
	err = provider.Health()
	if err != nil {
		t.Logf("Expected timeout but got: %v", err)
	}
}