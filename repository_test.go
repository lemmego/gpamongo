package gpamongo

import (
	"context"
	"os"
	"testing"

	"github.com/lemmego/gpa"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TestDoc struct {
	ID   primitive.ObjectID `bson:"_id,omitempty"`
	Name string             `bson:"name"`
	Age  int                `bson:"age"`
	Tags []string           `bson:"tags,omitempty"`
}

func setupTestRepository(t *testing.T) (*Repository[TestDoc], func()) {
	// Check for MongoDB connection string in environment
	mongoURL := os.Getenv("MONGODB_TEST_URL")
	if mongoURL == "" {
		mongoURL = "mongodb://localhost:27017"
	}

	config := gpa.Config{
		Driver:        "mongodb",
		ConnectionURL: mongoURL,
		Database:      "gpa_test",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Skipf("Skipping MongoDB tests: %v", err)
	}

	// Get collection and clean it
	collectionName := getCollectionName(TestDoc{})
	collection := provider.database.Collection(collectionName)
	collection.Drop(context.Background())

	repo := NewRepository[TestDoc](collection, provider)

	cleanup := func() {
		collection.Drop(context.Background())
		provider.Close()
	}

	return repo, cleanup
}

func TestRepositoryCreate(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()
	doc := &TestDoc{
		Name: "John Doe",
		Age:  30,
		Tags: []string{"developer", "golang"},
	}

	err := repo.Create(ctx, doc)
	if err != nil {
		t.Errorf("Failed to create document: %v", err)
	}

	if doc.ID.IsZero() {
		t.Error("Expected document ID to be set after creation")
	}
}

func TestRepositoryCreateBatch(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()
	docs := []*TestDoc{
		{Name: "User 1", Age: 25, Tags: []string{"frontend"}},
		{Name: "User 2", Age: 30, Tags: []string{"backend"}},
		{Name: "User 3", Age: 35, Tags: []string{"fullstack"}},
	}

	err := repo.CreateBatch(ctx, docs)
	if err != nil {
		t.Errorf("Failed to create batch: %v", err)
	}

	// Verify all documents have IDs
	for i, doc := range docs {
		if doc.ID.IsZero() {
			t.Errorf("Expected document %d to have ID set", i)
		}
	}
}

func TestRepositoryFindByID(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create a document first
	doc := &TestDoc{
		Name: "John Doe",
		Age:  30,
		Tags: []string{"developer"},
	}
	err := repo.Create(ctx, doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Find by ID
	found, err := repo.FindByID(ctx, doc.ID)
	if err != nil {
		t.Errorf("Failed to find document by ID: %v", err)
		return
	}

	if found == nil {
		t.Error("Expected found document to not be nil")
		return
	}

	if found.Name != doc.Name {
		t.Errorf("Expected name '%s', got '%s'", doc.Name, found.Name)
	}
	if found.Age != doc.Age {
		t.Errorf("Expected age %d, got %d", doc.Age, found.Age)
	}
}

func TestRepositoryFindByIDNotFound(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()
	nonExistentID := primitive.NewObjectID()

	_, err := repo.FindByID(ctx, nonExistentID)
	if err == nil {
		t.Error("Expected error for non-existent document")
	}

	if !gpa.IsErrorType(err, gpa.ErrorTypeNotFound) {
		t.Errorf("Expected not found error, got %v", err)
	}
}

func TestRepositoryFindAll(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test documents
	docs := []*TestDoc{
		{Name: "Alice", Age: 25, Tags: []string{"developer"}},
		{Name: "Bob", Age: 30, Tags: []string{"designer"}},
		{Name: "Charlie", Age: 35, Tags: []string{"manager"}},
	}

	for _, doc := range docs {
		err := repo.Create(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Find all documents
	found, err := repo.FindAll(ctx)
	if err != nil {
		t.Errorf("Failed to find all documents: %v", err)
	}

	if len(found) != 3 {
		t.Errorf("Expected 3 documents, got %d", len(found))
	}
}

func TestRepositoryFindAllWithOptions(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test documents
	docs := []*TestDoc{
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 30},
		{Name: "Charlie", Age: 35},
	}

	for _, doc := range docs {
		err := repo.Create(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Find with conditions
	found, err := repo.FindAll(ctx,
		gpa.Where("age", gpa.OpGreaterThan, 25),
		gpa.OrderBy("age", gpa.OrderAsc),
		gpa.Limit(2),
	)
	if err != nil {
		t.Errorf("Failed to find documents with options: %v", err)
	}

	if len(found) != 2 {
		t.Errorf("Expected 2 documents, got %d", len(found))
	}

	if found[0].Age != 30 {
		t.Errorf("Expected first document age 30, got %d", found[0].Age)
	}
	if found[1].Age != 35 {
		t.Errorf("Expected second document age 35, got %d", found[1].Age)
	}
}

func TestRepositoryUpdate(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create a document
	doc := &TestDoc{
		Name: "John Doe",
		Age:  30,
		Tags: []string{"developer"},
	}
	err := repo.Create(ctx, doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Update the document
	doc.Name = "John Smith"
	doc.Age = 31
	doc.Tags = []string{"senior developer"}
	err = repo.Update(ctx, doc)
	if err != nil {
		t.Errorf("Failed to update document: %v", err)
	}

	// Verify the update
	found, err := repo.FindByID(ctx, doc.ID)
	if err != nil {
		t.Fatalf("Failed to find updated document: %v", err)
	}

	if found.Name != "John Smith" {
		t.Errorf("Expected name 'John Smith', got '%s'", found.Name)
	}
	if found.Age != 31 {
		t.Errorf("Expected age 31, got %d", found.Age)
	}
}

func TestRepositoryUpdatePartial(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create a document
	doc := &TestDoc{
		Name: "John Doe",
		Age:  30,
		Tags: []string{"developer"},
	}
	err := repo.Create(ctx, doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Partial update
	updates := map[string]interface{}{
		"age": 31,
	}
	err = repo.UpdatePartial(ctx, doc.ID, updates)
	if err != nil {
		t.Errorf("Failed to update document partially: %v", err)
	}

	// Verify the update
	found, err := repo.FindByID(ctx, doc.ID)
	if err != nil {
		t.Fatalf("Failed to find updated document: %v", err)
	}

	if found.Age != 31 {
		t.Errorf("Expected age 31, got %d", found.Age)
	}
	if found.Name != "John Doe" {
		t.Errorf("Expected name unchanged, got '%s'", found.Name)
	}
}

func TestRepositoryDelete(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create a document
	doc := &TestDoc{
		Name: "John Doe",
		Age:  30,
	}
	err := repo.Create(ctx, doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Delete the document
	err = repo.Delete(ctx, doc.ID)
	if err != nil {
		t.Errorf("Failed to delete document: %v", err)
	}

	// Verify deletion
	_, err = repo.FindByID(ctx, doc.ID)
	if err == nil {
		t.Error("Expected error when finding deleted document")
	}
}

func TestRepositoryDeleteByCondition(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test documents
	docs := []*TestDoc{
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 30},
		{Name: "Charlie", Age: 35},
	}

	for _, doc := range docs {
		err := repo.Create(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Delete documents older than 25
	condition := gpa.BasicCondition{
		FieldName: "age",
		Op:        gpa.OpGreaterThan,
		Val:       25,
	}
	err := repo.DeleteByCondition(ctx, condition)
	if err != nil {
		t.Errorf("Failed to delete by condition: %v", err)
	}

	// Verify only Alice remains
	remaining, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("Failed to find remaining documents: %v", err)
	}

	if len(remaining) != 1 {
		t.Errorf("Expected 1 remaining document, got %d", len(remaining))
	}
	if remaining[0].Name != "Alice" {
		t.Errorf("Expected Alice to remain, got '%s'", remaining[0].Name)
	}
}

func TestRepositoryQuery(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test documents
	docs := []*TestDoc{
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 30},
		{Name: "Charlie", Age: 35},
	}

	for _, doc := range docs {
		err := repo.Create(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Query with complex conditions
	results, err := repo.Query(ctx,
		gpa.Where("age", gpa.OpGreaterThanOrEqual, 30),
		gpa.OrderBy("name", gpa.OrderAsc),
	)
	if err != nil {
		t.Errorf("Failed to query documents: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	if results[0].Name != "Bob" {
		t.Errorf("Expected first result to be Bob, got '%s'", results[0].Name)
	}
}

func TestRepositoryQueryOne(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create a document
	doc := &TestDoc{
		Name: "John Doe",
		Age:  30,
	}
	err := repo.Create(ctx, doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Query one
	found, err := repo.QueryOne(ctx, gpa.Where("name", gpa.OpEqual, "John Doe"))
	if err != nil {
		t.Errorf("Failed to query one document: %v", err)
	}

	if found.Name != doc.Name {
		t.Errorf("Expected name '%s', got '%s'", doc.Name, found.Name)
	}
}

func TestRepositoryCount(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test documents
	docs := []*TestDoc{
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 30},
		{Name: "Charlie", Age: 35},
	}

	for _, doc := range docs {
		err := repo.Create(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Count all documents
	count, err := repo.Count(ctx)
	if err != nil {
		t.Errorf("Failed to count documents: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}

	// Count with condition
	count, err = repo.Count(ctx, gpa.Where("age", gpa.OpGreaterThan, 25))
	if err != nil {
		t.Errorf("Failed to count documents with condition: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

func TestRepositoryExists(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Check non-existent document
	exists, err := repo.Exists(ctx, gpa.Where("name", gpa.OpEqual, "nonexistent"))
	if err != nil {
		t.Errorf("Failed to check existence: %v", err)
	}
	if exists {
		t.Error("Expected document not to exist")
	}

	// Create a document
	doc := &TestDoc{
		Name: "John Doe",
		Age:  30,
	}
	err = repo.Create(ctx, doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Check existing document
	exists, err = repo.Exists(ctx, gpa.Where("name", gpa.OpEqual, "John Doe"))
	if err != nil {
		t.Errorf("Failed to check existence: %v", err)
	}
	if !exists {
		t.Error("Expected document to exist")
	}
}

func TestRepositoryTransaction(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Successful transaction
	err := repo.Transaction(ctx, func(tx gpa.Transaction[TestDoc]) error {
		doc1 := &TestDoc{Name: "Doc 1", Age: 25}
		doc2 := &TestDoc{Name: "Doc 2", Age: 30}

		if err := tx.Create(ctx, doc1); err != nil {
			return err
		}
		if err := tx.Create(ctx, doc2); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		t.Errorf("Transaction failed: %v", err)
	}

	// Verify both documents were created
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count documents: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 documents after transaction, got %d", count)
	}
}

func TestRepositoryRawQuery(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	// Raw queries are not supported in MongoDB
	ctx := context.Background()
	_, err := repo.RawQuery(ctx, "SELECT * FROM docs", []interface{}{})
	if err == nil {
		t.Error("Expected error for unsupported raw query")
	}

	if !gpa.IsErrorType(err, gpa.ErrorTypeUnsupported) {
		t.Errorf("Expected unsupported error, got %v", err)
	}
}

func TestRepositoryRawExec(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	// Raw exec is not supported in MongoDB
	ctx := context.Background()
	_, err := repo.RawExec(ctx, "UPDATE docs SET age = 31", []interface{}{})
	if err == nil {
		t.Error("Expected error for unsupported raw exec")
	}

	if !gpa.IsErrorType(err, gpa.ErrorTypeUnsupported) {
		t.Errorf("Expected unsupported error, got %v", err)
	}
}

func TestRepositoryGetEntityInfo(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	info, err := repo.GetEntityInfo()
	if err != nil {
		t.Errorf("Failed to get entity info: %v", err)
	}

	if info.Name != "TestDoc" {
		t.Errorf("Expected entity name 'TestDoc', got '%s'", info.Name)
	}
	if len(info.Fields) == 0 {
		t.Error("Expected fields to be populated")
	}

	// Check for ID field
	var idField *gpa.FieldInfo
	for i, field := range info.Fields {
		if field.Name == "ID" {
			idField = &info.Fields[i]
			break
		}
	}
	if idField == nil {
		t.Error("Expected ID field to be found")
	} else if !idField.IsPrimaryKey {
		t.Error("Expected ID field to be primary key")
	}
}

func TestRepositoryClose(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	err := repo.Close()
	if err != nil {
		t.Errorf("Failed to close repository: %v", err)
	}
}

func TestRepositoryFindByDocument(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test documents
	docs := []*TestDoc{
		{Name: "Alice", Age: 25, Tags: []string{"developer"}},
		{Name: "Bob", Age: 30, Tags: []string{"designer"}},
	}

	for _, doc := range docs {
		err := repo.Create(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Find by document structure
	query := map[string]interface{}{
		"age": map[string]interface{}{"$gte": 30},
	}
	found, err := repo.FindByDocument(ctx, query)
	if err != nil {
		t.Errorf("Failed to find by document: %v", err)
	}

	if len(found) != 1 {
		t.Errorf("Expected 1 document, got %d", len(found))
	}
	if found[0].Name != "Bob" {
		t.Errorf("Expected Bob, got '%s'", found[0].Name)
	}
}

func TestRepositoryUpdateDocument(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create a document
	doc := &TestDoc{
		Name: "John Doe",
		Age:  30,
	}
	err := repo.Create(ctx, doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Update using document operations
	update := map[string]interface{}{
		"$set": map[string]interface{}{
			"age": 31,
		},
	}
	count, err := repo.UpdateDocument(ctx, doc.ID, update)
	if err != nil {
		t.Errorf("Failed to update document: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 document updated, got %d", count)
	}

	// Verify the update
	found, err := repo.FindByID(ctx, doc.ID)
	if err != nil {
		t.Fatalf("Failed to find updated document: %v", err)
	}

	if found.Age != 31 {
		t.Errorf("Expected age 31, got %d", found.Age)
	}
}

func TestRepositoryUpdateManyDocuments(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test documents
	docs := []*TestDoc{
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 30},
		{Name: "Charlie", Age: 35},
	}

	for _, doc := range docs {
		err := repo.Create(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Update many documents
	filter := map[string]interface{}{
		"age": map[string]interface{}{"$gte": 30},
	}
	update := map[string]interface{}{
		"$inc": map[string]interface{}{
			"age": 1,
		},
	}
	count, err := repo.UpdateManyDocuments(ctx, filter, update)
	if err != nil {
		t.Errorf("Failed to update many documents: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 documents updated, got %d", count)
	}
}

func TestRepositoryCreateIndex(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create an index
	keys := map[string]interface{}{
		"name": 1,
		"age":  -1,
	}
	err := repo.CreateIndex(ctx, keys, false)
	if err != nil {
		t.Errorf("Failed to create index: %v", err)
	}
}

func TestRepositoryDropIndex(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create an index first
	keys := map[string]interface{}{
		"name": 1,
	}
	err := repo.CreateIndex(ctx, keys, false)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Drop the index
	err = repo.DropIndex(ctx, "name_1")
	if err != nil {
		t.Errorf("Failed to drop index: %v", err)
	}
}

func TestRepositoryAggregate(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test documents
	docs := []*TestDoc{
		{Name: "Alice", Age: 25, Tags: []string{"developer"}},
		{Name: "Bob", Age: 30, Tags: []string{"developer"}},
		{Name: "Charlie", Age: 35, Tags: []string{"manager"}},
	}

	for _, doc := range docs {
		err := repo.Create(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Aggregate by tags
	pipeline := []map[string]interface{}{
		{
			"$unwind": "$tags",
		},
		{
			"$group": map[string]interface{}{
				"_id":   "$tags",
				"count": map[string]interface{}{"$sum": 1},
			},
		},
	}

	results, err := repo.Aggregate(ctx, pipeline)
	if err != nil {
		t.Errorf("Failed to aggregate: %v", err)
	}

	if len(results) < 1 {
		t.Error("Expected aggregation results")
	}
}

func TestRepositoryDistinct(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test documents
	docs := []*TestDoc{
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 30},
		{Name: "Charlie", Age: 25},
	}

	for _, doc := range docs {
		err := repo.Create(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Get distinct ages
	values, err := repo.Distinct(ctx, "age", map[string]interface{}{})
	if err != nil {
		t.Errorf("Failed to get distinct values: %v", err)
	}

	if len(values) != 2 {
		t.Errorf("Expected 2 distinct ages, got %d", len(values))
	}
}

func TestConvertToObjectID(t *testing.T) {
	// Test with ObjectID
	original := primitive.NewObjectID()
	converted, err := convertToObjectID(original)
	if err != nil {
		t.Errorf("Failed to convert ObjectID: %v", err)
	}
	if converted != original {
		t.Error("Expected ObjectID to remain unchanged")
	}

	// Test with string
	hexString := "507f1f77bcf86cd799439011"
	converted, err = convertToObjectID(hexString)
	if err != nil {
		t.Errorf("Failed to convert string: %v", err)
	}
	if converted.Hex() != hexString {
		t.Errorf("Expected hex %s, got %s", hexString, converted.Hex())
	}

	// Test with invalid type
	_, err = convertToObjectID(123)
	if err == nil {
		t.Error("Expected error for invalid type")
	}
}

func TestExtractID(t *testing.T) {
	doc := TestDoc{
		ID:   primitive.NewObjectID(),
		Name: "Test",
		Age:  30,
	}

	id, err := extractID(&doc)
	if err != nil {
		t.Errorf("Failed to extract ID: %v", err)
	}

	if id != doc.ID {
		t.Error("Expected extracted ID to match document ID")
	}

	// Test with struct without ID
	type NoID struct {
		Name string
	}
	noIDDoc := NoID{Name: "Test"}
	_, err = extractID(&noIDDoc)
	if err == nil {
		t.Error("Expected error for struct without ID")
	}
}