package gpamongo

import (
	"strings"

	"github.com/lemmego/gpa"
	"go.mongodb.org/mongo-driver/mongo"
)

// =====================================
// Error Conversion
// =====================================

// convertMongoError converts MongoDB errors to GPA errors
func convertMongoError(err error) error {
	if err == nil {
		return nil
	}

	switch err {
	case mongo.ErrNoDocuments:
		return gpa.GPAError{
			Type:    gpa.ErrorTypeNotFound,
			Message: "document not found",
			Cause:   err,
		}
	case mongo.ErrNilDocument:
		return gpa.GPAError{
			Type:    gpa.ErrorTypeValidation,
			Message: "nil document provided",
			Cause:   err,
		}
	case mongo.ErrNilValue:
		return gpa.GPAError{
			Type:    gpa.ErrorTypeValidation,
			Message: "nil value provided",
			Cause:   err,
		}
	}

	// Check for MongoDB-specific errors
	if mongoErr, ok := err.(mongo.WriteException); ok {
		for _, writeErr := range mongoErr.WriteErrors {
			switch writeErr.Code {
			case 11000, 11001: // Duplicate key error
				return gpa.GPAError{
					Type:    gpa.ErrorTypeDuplicate,
					Message: "duplicate key violation",
					Cause:   err,
				}
			case 121: // Document validation failed
				return gpa.GPAError{
					Type:    gpa.ErrorTypeValidation,
					Message: "document validation failed",
					Cause:   err,
				}
			}
		}
	}

	// Check for bulk write errors
	if bulkErr, ok := err.(mongo.BulkWriteException); ok {
		for _, writeErr := range bulkErr.WriteErrors {
			switch writeErr.Code {
			case 11000, 11001: // Duplicate key error
				return gpa.GPAError{
					Type:    gpa.ErrorTypeDuplicate,
					Message: "duplicate key violation in bulk operation",
					Cause:   err,
				}
			}
		}
	}

	// Check for command errors
	if cmdErr, ok := err.(mongo.CommandError); ok {
		switch cmdErr.Code {
		case 11000, 11001: // Duplicate key error
			return gpa.GPAError{
				Type:    gpa.ErrorTypeDuplicate,
				Message: "duplicate key violation",
				Cause:   err,
			}
		case 26: // NamespaceNotFound
			return gpa.GPAError{
				Type:    gpa.ErrorTypeNotFound,
				Message: "collection not found",
				Cause:   err,
			}
		case 48: // CollectionNotFound
			return gpa.GPAError{
				Type:    gpa.ErrorTypeNotFound,
				Message: "collection not found",
				Cause:   err,
			}
		case 13: // Unauthorized
			return gpa.GPAError{
				Type:    gpa.ErrorTypeConnection,
				Message: "unauthorized access",
				Cause:   err,
			}
		case 18: // AuthenticationFailed
			return gpa.GPAError{
				Type:    gpa.ErrorTypeConnection,
				Message: "authentication failed",
				Cause:   err,
			}
		case 251: // NoSuchTransaction
			return gpa.GPAError{
				Type:    gpa.ErrorTypeTransaction,
				Message: "transaction not found",
				Cause:   err,
			}
		case 244: // TransactionTooOld
			return gpa.GPAError{
				Type:    gpa.ErrorTypeTransaction,
				Message: "transaction too old",
				Cause:   err,
			}
		}
	}

	// Check for network timeout errors
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return gpa.GPAError{
			Type:    gpa.ErrorTypeTimeout,
			Message: "operation timeout",
			Cause:   err,
		}
	}

	// Check for connection errors
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "network") {
		return gpa.GPAError{
			Type:    gpa.ErrorTypeConnection,
			Message: "connection error",
			Cause:   err,
		}
	}

	// Check for validation errors
	if strings.Contains(errStr, "validation") || strings.Contains(errStr, "invalid") {
		return gpa.GPAError{
			Type:    gpa.ErrorTypeValidation,
			Message: "validation error",
			Cause:   err,
		}
	}

	// Default to generic error
	return gpa.GPAError{
		Type:    gpa.ErrorTypeConnection,
		Message: "database operation failed",
		Cause:   err,
	}
}

// =====================================
// Specific Error Constructors
// =====================================

// NewNotFoundError creates a not found error
func NewNotFoundError(message string) error {
	return gpa.GPAError{
		Type:    gpa.ErrorTypeNotFound,
		Message: message,
	}
}

// NewValidationError creates a validation error
func NewValidationError(message string) error {
	return gpa.GPAError{
		Type:    gpa.ErrorTypeValidation,
		Message: message,
	}
}

// NewDuplicateError creates a duplicate error
func NewDuplicateError(message string) error {
	return gpa.GPAError{
		Type:    gpa.ErrorTypeDuplicate,
		Message: message,
	}
}

// NewConnectionError creates a connection error
func NewConnectionError(message string, cause error) error {
	return gpa.GPAError{
		Type:    gpa.ErrorTypeConnection,
		Message: message,
		Cause:   cause,
	}
}

// NewTransactionError creates a transaction error
func NewTransactionError(message string, cause error) error {
	return gpa.GPAError{
		Type:    gpa.ErrorTypeTransaction,
		Message: message,
		Cause:   cause,
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string, cause error) error {
	return gpa.GPAError{
		Type:    gpa.ErrorTypeTimeout,
		Message: message,
		Cause:   cause,
	}
}

// NewUnsupportedError creates an unsupported operation error
func NewUnsupportedError(message string) error {
	return gpa.GPAError{
		Type:    gpa.ErrorTypeUnsupported,
		Message: message,
	}
}

// =====================================
// Registration
// =====================================

// Legacy registration removed - use NewProvider() instead