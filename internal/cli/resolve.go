package cli

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// resolveDocumentID resolves a document identifier to a UUID.
// It tries, in order: full UUID, UUID prefix (min 8 hex chars), file path.
func resolveDocumentID(ctx context.Context, identifier string) (uuid.UUID, error) {
	// Try full UUID
	id, parseErr := uuid.Parse(identifier)
	if parseErr == nil {
		_, err := store.GetDocumentByID(ctx, id)
		if err != nil {
			return uuid.Nil, fmt.Errorf("document not found: %s", identifier)
		}
		return id, nil
	}

	// Try as UUID prefix (at least 8 hex chars)
	if isHexString(identifier) && len(identifier) >= 8 {
		doc, err := store.GetDocumentByIDPrefix(ctx, identifier)
		if err != nil {
			return uuid.Nil, fmt.Errorf("document not found: %s", identifier)
		}
		return doc.ID, nil
	}

	// Try as file path
	doc, err := store.GetDocumentByPath(ctx, identifier)
	if err != nil {
		return uuid.Nil, fmt.Errorf("document not found: %s", identifier)
	}
	return doc.ID, nil
}

// isHexString returns true if s contains only hexadecimal characters and hyphens.
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-') {
			return false
		}
	}
	return len(s) > 0
}
