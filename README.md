# gpamongo

A MongoDB adapter for the Go Persistence API (GPA), providing type-safe database operations with generic support.

## Features

- 🔐 **Type-Safe Operations**: Full compile-time type safety using Go generics
- 📊 **Complete CRUD Support**: Create, Read, Update, Delete operations with batch support
- 🗂️ **Document Operations**: Native MongoDB document-style operations
- 🔍 **Advanced Queries**: Support for complex queries, aggregation, and geospatial operations
- 🔄 **Transactions**: Full transaction support with type safety
- 📑 **Text Search**: Full-text search capabilities
- 🗺️ **Geospatial Queries**: Geographic queries with `$near` and `$geoWithin`
- 📈 **Aggregation Pipeline**: Native MongoDB aggregation support
- 🏗️ **Index Management**: Create and manage database indexes
- 🔧 **Connection Management**: Robust connection handling with health checks

## Installation

```bash
go get github.com/lemmego/gpamongo
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/lemmego/gpa"
    "github.com/lemmego/gpamongo"
)

type User struct {
    ID    primitive.ObjectID `bson:"_id,omitempty"`
    Name  string            `bson:"name"`
    Email string            `bson:"email"`
}

func main() {
    // Configure connection
    config := gpa.Config{
        Host:     "localhost",
        Port:     27017,
        Database: "myapp",
        Username: "user",
        Password: "password",
    }
    
    // Create provider
    provider, err := gpamongo.NewProvider(config)
    if err != nil {
        log.Fatal(err)
    }
    defer provider.Close()
    
    // Get type-safe repository
    userRepo := gpamongo.GetRepository[User](provider)
    
    // Create a new user
    user := &User{
        Name:  "John Doe",
        Email: "john@example.com",
    }
    
    ctx := context.Background()
    if err := userRepo.Create(ctx, user); err != nil {
        log.Fatal(err)
    }
    
    // Find user by ID
    foundUser, err := userRepo.FindByID(ctx, user.ID)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Found user: %+v", foundUser)
}
```

## Usage

### Basic Operations

```go
// Create
user := &User{Name: "Alice", Email: "alice@example.com"}
err := userRepo.Create(ctx, user)

// Read
user, err := userRepo.FindByID(ctx, userID)
users, err := userRepo.FindAll(ctx)

// Update
user.Name = "Alice Smith"
err := userRepo.Update(ctx, user)

// Delete
err := userRepo.Delete(ctx, userID)
```

### Batch Operations

```go
// Create multiple users
users := []*User{
    {Name: "User 1", Email: "user1@example.com"},
    {Name: "User 2", Email: "user2@example.com"},
}
err := userRepo.CreateBatch(ctx, users)
```

### Query Operations

```go
// Query with conditions
users, err := userRepo.Query(ctx,
    gpa.Where("name", gpa.OpEqual, "John"),
    gpa.OrderBy("email", "ASC"),
    gpa.Limit(10),
    gpa.Offset(0),
)

// Count documents
count, err := userRepo.Count(ctx, gpa.Where("active", gpa.OpEqual, true))
```

### Document Operations

```go
// Find by document structure
filter := map[string]interface{}{
    "status": "active",
    "age":    map[string]interface{}{"$gt": 18},
}
users, err := userRepo.FindByDocument(ctx, filter)

// Update with MongoDB operators
update := map[string]interface{}{
    "$set": map[string]interface{}{
        "lastLogin": time.Now(),
    },
    "$inc": map[string]interface{}{
        "loginCount": 1,
    },
}
count, err := userRepo.UpdateManyDocuments(ctx, filter, update)
```

### Aggregation

```go
// Execute aggregation pipeline
pipeline := []map[string]interface{}{
    {"$match": map[string]interface{}{"status": "active"}},
    {"$group": map[string]interface{}{
        "_id":   "$department",
        "count": map[string]interface{}{"$sum": 1},
    }},
}
results, err := userRepo.Aggregate(ctx, pipeline)
```

### Text Search

```go
// Full-text search
users, err := userRepo.TextSearch(ctx, "john doe", gpa.Limit(10))
```

### Geospatial Queries

```go
// Find locations near a point
locations, err := locationRepo.FindNear(ctx, 
    "coordinates", 
    []float64{-73.9857, 40.7484}, // [longitude, latitude]
    1000, // max distance in meters
)

// Find within polygon
polygon := [][]float64{
    {-74.0, 40.7},
    {-73.9, 40.7},
    {-73.9, 40.8},
    {-74.0, 40.8},
    {-74.0, 40.7},
}
locations, err := locationRepo.FindWithinPolygon(ctx, "coordinates", polygon)
```

### Transactions

```go
err := userRepo.Transaction(ctx, func(txRepo gpa.Transaction[User]) error {
    // All operations within this function are part of the transaction
    user := &User{Name: "Transaction User", Email: "tx@example.com"}
    if err := txRepo.Create(ctx, user); err != nil {
        return err
    }
    
    // Update another user
    return txRepo.UpdatePartial(ctx, otherUserID, map[string]interface{}{
        "lastModified": time.Now(),
    })
})
```

### Index Management

```go
// Create single field index
err := userRepo.CreateIndex(ctx, map[string]interface{}{
    "email": 1,
}, true) // unique index

// Create compound index
err := userRepo.CreateIndex(ctx, map[string]interface{}{
    "name":   1,
    "status": -1,
}, false)

// Create text index for full-text search
err := userRepo.CreateIndex(ctx, map[string]interface{}{
    "name":        "text",
    "description": "text",
}, false)
```

## Configuration

### Connection Options

```go
config := gpa.Config{
    Host:          "localhost",
    Port:          27017,
    Database:      "myapp",
    Username:      "user",
    Password:      "password",
    ConnectionURL: "mongodb://localhost:27017/myapp", // Alternative to individual fields
    Options: map[string]interface{}{
        "mongo": map[string]interface{}{
            "max_pool_size": uint64(100),
            "min_pool_size": uint64(10),
            "max_idle_time": time.Minute * 5,
        },
    },
}
```

### Supported MongoDB Features

- ✅ Transactions
- ✅ Indexes (Single, Compound, Text, Geospatial)
- ✅ Full-text search
- ✅ Geospatial queries
- ✅ Aggregation pipelines
- ✅ Sharding support
- ✅ Replica sets

## Error Handling

The library provides structured error handling through the GPA error system:

```go
user, err := userRepo.FindByID(ctx, userID)
if err != nil {
    var gpaErr gpa.GPAError
    if errors.As(err, &gpaErr) {
        switch gpaErr.Type {
        case gpa.ErrorTypeNotFound:
            // Handle not found
        case gpa.ErrorTypeConnection:
            // Handle connection error
        case gpa.ErrorTypeValidation:
            // Handle validation error
        }
    }
}
```

## License

MIT License - see [LICENSE.md](LICENSE.md) for details.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Requirements

- Go 1.24.3 or later
- MongoDB 4.0 or later