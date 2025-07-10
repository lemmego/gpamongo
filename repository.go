// Package gpamongo provides a MongoDB adapter for the Go Persistence API (GPA)
package gpamongo

import (
	"context"
	"fmt"
	"reflect"

	"github.com/lemmego/gpa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// =====================================
// Generic MongoDB Repository Implementation
// =====================================

// RepositoryG implements type-safe MongoDB operations using Go generics.
// Provides compile-time type safety for all CRUD and document operations.
type Repository[T any] struct {
	collection *mongo.Collection
	provider   *Provider
}

// NewRepository creates a new generic MongoDB repository for type T.
// Example: userRepo := NewRepository[User](collection, provider)
func NewRepository[T any](collection *mongo.Collection, provider *Provider) *Repository[T] {
	return &Repository[T]{
		collection: collection,
		provider:   provider,
	}
}

// =====================================
// Repository[T] Implementation
// =====================================

// Create inserts a new entity with compile-time type safety.
func (r *Repository[T]) Create(ctx context.Context, entity *T) error {
	result, err := r.collection.InsertOne(ctx, entity)
	if err != nil {
		return convertMongoError(err)
	}
	
	// Set the ID on the entity if it was generated
	if result.InsertedID != nil {
		// Use reflection to set the ID field
		if err := setEntityID(entity, result.InsertedID); err != nil {
			// Log the error but don't fail the operation
			// The document was created successfully
		}
	}
	
	return nil
}

// CreateBatch inserts multiple entities with compile-time type safety.
func (r *Repository[T]) CreateBatch(ctx context.Context, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}
	
	// Convert []*T to []interface{}
	docs := make([]interface{}, len(entities))
	for i, entity := range entities {
		docs[i] = entity
	}
	
	result, err := r.collection.InsertMany(ctx, docs)
	if err != nil {
		return convertMongoError(err)
	}
	
	// Set the IDs on the entities if they were generated
	if result.InsertedIDs != nil && len(result.InsertedIDs) == len(entities) {
		for i, id := range result.InsertedIDs {
			if err := setEntityID(entities[i], id); err != nil {
				// Log the error but don't fail the operation
				// The documents were created successfully
			}
		}
	}
	
	return nil
}

// FindByID retrieves a single entity by ID with compile-time type safety.
func (r *Repository[T]) FindByID(ctx context.Context, id interface{}) (*T, error) {
	objectID, err := convertToObjectID(id)
	if err != nil {
		return nil, err
	}
	
	var entity T
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&entity)
	if err != nil {
		return nil, convertMongoError(err)
	}
	return &entity, nil
}

// FindAll retrieves all entities with compile-time type safety.
func (r *Repository[T]) FindAll(ctx context.Context, opts ...gpa.QueryOption) ([]*T, error) {
	filter, findOptions := r.buildQuery(opts...)
	
	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, convertMongoError(err)
	}
	defer cursor.Close(ctx)
	
	var entities []*T
	for cursor.Next(ctx) {
		var entity T
		if err := cursor.Decode(&entity); err != nil {
			return nil, convertMongoError(err)
		}
		entities = append(entities, &entity)
	}
	
	if err := cursor.Err(); err != nil {
		return nil, convertMongoError(err)
	}
	
	return entities, nil
}

// Update modifies an existing entity with compile-time type safety.
func (r *Repository[T]) Update(ctx context.Context, entity *T) error {
	id, err := extractID(entity)
	if err != nil {
		return err
	}
	
	objectID, err := convertToObjectID(id)
	if err != nil {
		return err
	}
	
	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": objectID}, entity)
	if err != nil {
		return convertMongoError(err)
	}
	
	if result.MatchedCount == 0 {
		return gpa.GPAError{
			Type:    gpa.ErrorTypeNotFound,
			Message: "entity not found",
		}
	}
	
	return nil
}

// UpdatePartial modifies specific fields of an entity.
func (r *Repository[T]) UpdatePartial(ctx context.Context, id interface{}, updates map[string]interface{}) error {
	objectID, err := convertToObjectID(id)
	if err != nil {
		return err
	}
	
	updateDoc := bson.M{"$set": updates}
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": objectID}, updateDoc)
	if err != nil {
		return convertMongoError(err)
	}
	
	if result.MatchedCount == 0 {
		return gpa.GPAError{
			Type:    gpa.ErrorTypeNotFound,
			Message: "entity not found",
		}
	}
	
	return nil
}

// Delete removes an entity by ID with compile-time type safety.
func (r *Repository[T]) Delete(ctx context.Context, id interface{}) error {
	objectID, err := convertToObjectID(id)
	if err != nil {
		return err
	}
	
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return convertMongoError(err)
	}
	
	if result.DeletedCount == 0 {
		return gpa.GPAError{
			Type:    gpa.ErrorTypeNotFound,
			Message: "entity not found",
		}
	}
	
	return nil
}

// DeleteByCondition removes entities matching a condition.
func (r *Repository[T]) DeleteByCondition(ctx context.Context, condition gpa.Condition) error {
	filter := r.buildConditionFilter(condition)
	_, err := r.collection.DeleteMany(ctx, filter)
	return convertMongoError(err)
}

// Query retrieves entities based on query options with compile-time type safety.
func (r *Repository[T]) Query(ctx context.Context, opts ...gpa.QueryOption) ([]*T, error) {
	return r.FindAll(ctx, opts...)
}

// QueryOne retrieves a single entity based on query options.
func (r *Repository[T]) QueryOne(ctx context.Context, opts ...gpa.QueryOption) (*T, error) {
	filter, findOptions := r.buildQuery(opts...)
	findOptions.SetLimit(1)
	
	var entity T
	err := r.collection.FindOne(ctx, filter).Decode(&entity)
	if err != nil {
		return nil, convertMongoError(err)
	}
	return &entity, nil
}

// Count returns the number of entities matching query options.
func (r *Repository[T]) Count(ctx context.Context, opts ...gpa.QueryOption) (int64, error) {
	filter, _ := r.buildQuery(opts...)
	count, err := r.collection.CountDocuments(ctx, filter)
	return count, convertMongoError(err)
}

// Exists checks if any entity matches the query options.
func (r *Repository[T]) Exists(ctx context.Context, opts ...gpa.QueryOption) (bool, error) {
	count, err := r.Count(ctx, opts...)
	return count > 0, err
}

// Transaction executes a function within a transaction with type safety.
func (r *Repository[T]) Transaction(ctx context.Context, fn gpa.TransactionFunc[T]) error {
	session, err := r.provider.client.StartSession()
	if err != nil {
		return convertMongoError(err)
	}
	defer session.EndSession(ctx)
	
	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		txRepo := &Transaction[T]{
			Repository: &Repository[T]{
				collection: r.collection,
				provider:   r.provider,
			},
		}
		return nil, fn(txRepo)
	}
	
	_, err = session.WithTransaction(ctx, callback)
	return convertMongoError(err)
}

// RawQuery executes a raw MongoDB query with compile-time type safety.
func (r *Repository[T]) RawQuery(ctx context.Context, query string, args []interface{}) ([]*T, error) {
	// MongoDB doesn't use SQL, so we'll interpret this as a pipeline
	// This is a simplified implementation - in practice, you'd want more sophisticated parsing
	return nil, gpa.GPAError{
		Type:    gpa.ErrorTypeUnsupported,
		Message: "raw SQL queries not supported in MongoDB - use Aggregate instead",
	}
}

// RawExec executes a raw MongoDB command.
func (r *Repository[T]) RawExec(ctx context.Context, query string, args []interface{}) (gpa.Result, error) {
	return nil, gpa.GPAError{
		Type:    gpa.ErrorTypeUnsupported,
		Message: "raw SQL execution not supported in MongoDB",
	}
}

// GetEntityInfo returns metadata about entity type T.
func (r *Repository[T]) GetEntityInfo() (*gpa.EntityInfo, error) {
	var zero T
	entityType := reflect.TypeOf(zero)
	
	info := &gpa.EntityInfo{
		Name:      entityType.Name(),
		TableName: r.collection.Name(),
		Fields:    make([]gpa.FieldInfo, 0),
	}
	
	// Analyze struct fields
	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		
		fieldInfo := gpa.FieldInfo{
			Name:         field.Name,
			Type:         field.Type,
			DatabaseType: "bson",
			Tag:          string(field.Tag),
		}
		
		// Check for MongoDB-specific tags
		if bsonTag := field.Tag.Get("bson"); bsonTag != "" {
			if bsonTag == "_id" || bsonTag == "_id,omitempty" {
				fieldInfo.IsPrimaryKey = true
				info.PrimaryKey = append(info.PrimaryKey, field.Name)
			}
		}
		
		info.Fields = append(info.Fields, fieldInfo)
	}
	
	return info, nil
}

// Close closes the repository (no-op for MongoDB).
func (r *Repository[T]) Close() error {
	return nil
}

// =====================================
// DocumentRepository[T] Implementation
// =====================================

// FindByDocument finds documents that match the given document structure.
func (r *Repository[T]) FindByDocument(ctx context.Context, document map[string]interface{}) ([]*T, error) {
	cursor, err := r.collection.Find(ctx, document)
	if err != nil {
		return nil, convertMongoError(err)
	}
	defer cursor.Close(ctx)
	
	var entities []*T
	for cursor.Next(ctx) {
		var entity T
		if err := cursor.Decode(&entity); err != nil {
			return nil, convertMongoError(err)
		}
		entities = append(entities, &entity)
	}
	
	return entities, convertMongoError(cursor.Err())
}

// UpdateDocument updates a document using document-style operations.
func (r *Repository[T]) UpdateDocument(ctx context.Context, id interface{}, update map[string]interface{}) (int64, error) {
	objectID, err := convertToObjectID(id)
	if err != nil {
		return 0, err
	}
	
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": objectID}, update)
	if err != nil {
		return 0, convertMongoError(err)
	}
	
	return result.ModifiedCount, nil
}

// UpdateManyDocuments updates multiple entities using document-style operations.
func (r *Repository[T]) UpdateManyDocuments(ctx context.Context, filter map[string]interface{}, update map[string]interface{}) (int64, error) {
	result, err := r.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, convertMongoError(err)
	}
	
	return result.ModifiedCount, nil
}

// ReplaceDocument completely replaces a document with new content.
func (r *Repository[T]) ReplaceDocument(ctx context.Context, id interface{}, entity *T) (*T, error) {
	objectID, err := convertToObjectID(id)
	if err != nil {
		return nil, err
	}
	
	opts := options.FindOneAndReplace().SetReturnDocument(options.After)
	
	var result T
	err = r.collection.FindOneAndReplace(ctx, bson.M{"_id": objectID}, entity, opts).Decode(&result)
	if err != nil {
		return nil, convertMongoError(err)
	}
	
	return &result, nil
}

// CreateCollection creates a new collection for entity type T.
func (r *Repository[T]) CreateCollection(ctx context.Context) error {
	return convertMongoError(r.provider.client.Database(r.collection.Database().Name()).CreateCollection(ctx, r.collection.Name()))
}

// DropCollection removes the entire collection for entity type T.
func (r *Repository[T]) DropCollection(ctx context.Context) error {
	return convertMongoError(r.collection.Drop(ctx))
}

// CreateIndex creates an index on the collection for entity type T.
func (r *Repository[T]) CreateIndex(ctx context.Context, keys map[string]interface{}, unique bool) error {
	// Convert map to bson.D for ordered keys
	var indexKeys bson.D
	for key, value := range keys {
		indexKeys = append(indexKeys, bson.E{Key: key, Value: value})
	}
	
	indexModel := mongo.IndexModel{
		Keys: indexKeys,
		Options: options.Index().SetUnique(unique),
	}
	
	_, err := r.collection.Indexes().CreateOne(ctx, indexModel)
	return convertMongoError(err)
}

// DropIndex removes an index by name.
func (r *Repository[T]) DropIndex(ctx context.Context, indexName string) error {
	_, err := r.collection.Indexes().DropOne(ctx, indexName)
	return convertMongoError(err)
}

// TextSearch performs full-text search on indexed text fields.
func (r *Repository[T]) TextSearch(ctx context.Context, query string, opts ...gpa.QueryOption) ([]*T, error) {
	filter := bson.M{"$text": bson.M{"$search": query}}
	
	// Build additional options
	findQuery := &gpa.Query{}
	for _, opt := range opts {
		opt.Apply(findQuery)
	}
	
	findOptions := options.Find()
	
	// Apply limit
	if findQuery.Limit != nil {
		findOptions.SetLimit(int64(*findQuery.Limit))
	}
	
	// Apply offset
	if findQuery.Offset != nil {
		findOptions.SetSkip(int64(*findQuery.Offset))
	}
	
	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, convertMongoError(err)
	}
	defer cursor.Close(ctx)
	
	var entities []*T
	for cursor.Next(ctx) {
		var entity T
		if err := cursor.Decode(&entity); err != nil {
			return nil, convertMongoError(err)
		}
		entities = append(entities, &entity)
	}
	
	return entities, convertMongoError(cursor.Err())
}

// FindNear finds entities near a geographical point.
func (r *Repository[T]) FindNear(ctx context.Context, field string, point []float64, maxDistance float64) ([]*T, error) {
	filter := bson.M{
		field: bson.M{
			"$near": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					"coordinates": point,
				},
				"$maxDistance": maxDistance,
			},
		},
	}
	
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, convertMongoError(err)
	}
	defer cursor.Close(ctx)
	
	var entities []*T
	for cursor.Next(ctx) {
		var entity T
		if err := cursor.Decode(&entity); err != nil {
			return nil, convertMongoError(err)
		}
		entities = append(entities, &entity)
	}
	
	return entities, convertMongoError(cursor.Err())
}

// FindWithinPolygon finds entities within a geographical polygon.
func (r *Repository[T]) FindWithinPolygon(ctx context.Context, field string, polygon [][]float64) ([]*T, error) {
	filter := bson.M{
		field: bson.M{
			"$geoWithin": bson.M{
				"$geometry": bson.M{
					"type":        "Polygon",
					"coordinates": [][]float64{polygon[0]}, // MongoDB expects array of LinearRings
				},
			},
		},
	}
	
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, convertMongoError(err)
	}
	defer cursor.Close(ctx)
	
	var entities []*T
	for cursor.Next(ctx) {
		var entity T
		if err := cursor.Decode(&entity); err != nil {
			return nil, convertMongoError(err)
		}
		entities = append(entities, &entity)
	}
	
	return entities, convertMongoError(cursor.Err())
}

// Aggregate executes an aggregation pipeline and returns typed results.
func (r *Repository[T]) Aggregate(ctx context.Context, pipeline []map[string]interface{}) ([]map[string]interface{}, error) {
	// Convert to bson.D pipeline
	bsonPipeline := make([]bson.D, len(pipeline))
	for i, stage := range pipeline {
		bsonStage := bson.D{}
		for key, value := range stage {
			bsonStage = append(bsonStage, bson.E{Key: key, Value: value})
		}
		bsonPipeline[i] = bsonStage
	}
	
	cursor, err := r.collection.Aggregate(ctx, bsonPipeline)
	if err != nil {
		return nil, convertMongoError(err)
	}
	defer cursor.Close(ctx)
	
	var results []map[string]interface{}
	for cursor.Next(ctx) {
		var result map[string]interface{}
		if err := cursor.Decode(&result); err != nil {
			return nil, convertMongoError(err)
		}
		results = append(results, result)
	}
	
	return results, convertMongoError(cursor.Err())
}

// Distinct returns distinct values for a specified field across the collection.
func (r *Repository[T]) Distinct(ctx context.Context, field string, filter map[string]interface{}) ([]interface{}, error) {
	values, err := r.collection.Distinct(ctx, field, filter)
	return values, convertMongoError(err)
}

// =====================================
// TransactionG Implementation
// =====================================

// Transaction implementation moved to transaction.go

// =====================================
// Helper Methods
// =====================================

// buildQuery builds MongoDB filter and find options from GPA query options
func (r *Repository[T]) buildQuery(opts ...gpa.QueryOption) (bson.M, *options.FindOptions) {
	query := &gpa.Query{}
	
	// Apply all options
	for _, opt := range opts {
		opt.Apply(query)
	}
	
	// Build filter
	filter := bson.M{}
	for _, condition := range query.Conditions {
		conditionFilter := r.buildConditionFilter(condition)
		for key, value := range conditionFilter {
			filter[key] = value
		}
	}
	
	// Build find options
	findOptions := options.Find()
	
	// Apply limit
	if query.Limit != nil {
		findOptions.SetLimit(int64(*query.Limit))
	}
	
	// Apply offset
	if query.Offset != nil {
		findOptions.SetSkip(int64(*query.Offset))
	}
	
	// Apply sorting
	if len(query.Orders) > 0 {
		sort := bson.D{}
		for _, order := range query.Orders {
			direction := 1
			if order.Direction == "DESC" {
				direction = -1
			}
			sort = append(sort, bson.E{Key: order.Field, Value: direction})
		}
		findOptions.SetSort(sort)
	}
	
	// Apply field projection
	if len(query.Fields) > 0 {
		projection := bson.M{}
		for _, field := range query.Fields {
			projection[field] = 1
		}
		findOptions.SetProjection(projection)
	}
	
	return filter, findOptions
}

// buildConditionFilter builds a MongoDB filter from a GPA condition
func (r *Repository[T]) buildConditionFilter(condition gpa.Condition) bson.M {
	// Basic implementation - can be enhanced later
	switch cond := condition.(type) {
	case gpa.BasicCondition:
		field := cond.Field()
		operator := cond.Operator()
		value := cond.Value()
		
		switch operator {
		case gpa.OpEqual:
			return bson.M{field: value}
		case gpa.OpNotEqual:
			return bson.M{field: bson.M{"$ne": value}}
		case gpa.OpGreaterThan:
			return bson.M{field: bson.M{"$gt": value}}
		case gpa.OpGreaterThanOrEqual:
			return bson.M{field: bson.M{"$gte": value}}
		case gpa.OpLessThan:
			return bson.M{field: bson.M{"$lt": value}}
		case gpa.OpLessThanOrEqual:
			return bson.M{field: bson.M{"$lte": value}}
		case gpa.OpIn:
			return bson.M{field: bson.M{"$in": value}}
		case gpa.OpNotIn:
			return bson.M{field: bson.M{"$nin": value}}
		default:
			return bson.M{field: value}
		}
	default:
		// For now, return empty filter for complex conditions
		return bson.M{}
	}
}

// Helper function to extract ID from entity
func extractID(entity interface{}) (interface{}, error) {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	// Look for ID field
	if v.Kind() == reflect.Struct {
		if idField := v.FieldByName("ID"); idField.IsValid() {
			return idField.Interface(), nil
		}
		// Look for _id field
		if idField := v.FieldByName("Id"); idField.IsValid() {
			return idField.Interface(), nil
		}
	}
	
	return nil, gpa.GPAError{
		Type:    gpa.ErrorTypeInvalidArgument,
		Message: "entity does not have an ID field",
	}
}

// Helper function to convert to ObjectID (reuse existing)
func convertToObjectID(id interface{}) (primitive.ObjectID, error) {
	switch v := id.(type) {
	case primitive.ObjectID:
		return v, nil
	case string:
		return primitive.ObjectIDFromHex(v)
	default:
		return primitive.NilObjectID, gpa.GPAError{
			Type:    gpa.ErrorTypeInvalidArgument,
			Message: "invalid ID type",
		}
	}
}

// =====================================
// Helper Functions
// =====================================

// setEntityID sets the ID field on an entity using reflection
func setEntityID(entity interface{}, id interface{}) error {
	v := reflect.ValueOf(entity)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("entity must be a pointer to a struct")
	}
	
	v = v.Elem()
	
	// Look for ID field (ID, Id, or _id)
	var idField reflect.Value
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		fieldName := field.Name
		bsonTag := field.Tag.Get("bson")
		
		if fieldName == "ID" || fieldName == "Id" || bsonTag == "_id" || bsonTag == "_id,omitempty" {
			idField = v.Field(i)
			break
		}
	}
	
	if !idField.IsValid() {
		return fmt.Errorf("no ID field found")
	}
	
	if !idField.CanSet() {
		return fmt.Errorf("ID field cannot be set")
	}
	
	// Convert the ID to the appropriate type
	idValue := reflect.ValueOf(id)
	if idField.Type() == idValue.Type() {
		idField.Set(idValue)
	} else if idField.Type() == reflect.TypeOf(primitive.ObjectID{}) {
		if objID, ok := id.(primitive.ObjectID); ok {
			idField.Set(reflect.ValueOf(objID))
		}
	}
	
	return nil
}

// =====================================
// Compile-time Interface Checks
// =====================================

var (
	_ gpa.Repository[any]     = (*Repository[any])(nil)
	_ gpa.DocumentRepository[any] = (*Repository[any])(nil)
	_ gpa.Transaction[any]    = (*Transaction[any])(nil)
)