package gpamongo

import (
	"context"

	"github.com/lemmego/gpa"
	"go.mongodb.org/mongo-driver/mongo"
)

// =====================================
// Transaction Implementation
// =====================================

// Transaction implements gpa.Transaction for MongoDB
type Transaction[T any] struct {
	*Repository[T]
	session mongo.Session
}

// Commit commits the transaction
func (t *Transaction[T]) Commit() error {
	// MongoDB transactions are committed in the Transaction method
	// This is a no-op as the commit happens automatically
	return nil
}

// Rollback rolls back the transaction
func (t *Transaction[T]) Rollback() error {
	// MongoDB transactions are rolled back automatically on error
	// This is a no-op as the rollback happens automatically
	return nil
}

// SetSavepoint sets a savepoint (not supported by MongoDB)
func (t *Transaction[T]) SetSavepoint(name string) error {
	return gpa.NewError(gpa.ErrorTypeUnsupported, "savepoints not supported by MongoDB")
}

// RollbackToSavepoint rolls back to a savepoint (not supported by MongoDB)
func (t *Transaction[T]) RollbackToSavepoint(name string) error {
	return gpa.NewError(gpa.ErrorTypeUnsupported, "savepoints not supported by MongoDB")
}

// Transaction executes a function within a nested transaction
func (t *Transaction[T]) Transaction(ctx context.Context, fn gpa.TransactionFunc[T]) error {
	// MongoDB doesn't support nested transactions
	// Execute the function with the current transaction
	return fn(t)
}