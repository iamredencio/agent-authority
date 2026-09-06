package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func newID() (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: generate id: %v", ErrInvalidInput, err)
	}
	return id, nil
}

func requireNonEmpty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s", ErrRequiredField, field)
	}
	return nil
}

func requireID(field string, id uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("%w: %s", ErrRequiredField, field)
	}
	return nil
}

func requireTime(field string, t time.Time) error {
	if t.IsZero() {
		return fmt.Errorf("%w: %s", ErrRequiredField, field)
	}
	return nil
}

func requireJSON(field string, raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return fmt.Errorf("%w: %s", ErrRequiredField, field)
	}
	if !json.Valid(raw) {
		return fmt.Errorf("%w: %s is not valid JSON", ErrInvalidInput, field)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%w: %s", ErrRequiredField, field)
	}
	return nil
}

func normalizeTime(t time.Time) time.Time {
	return t.UTC()
}

// SameOrganization fails closed when any related identifier is missing or
// belongs to a different tenant than org.
func SameOrganization(org uuid.UUID, others ...uuid.UUID) error {
	if org == uuid.Nil {
		return fmt.Errorf("%w: organization_id", ErrRequiredField)
	}
	for _, other := range others {
		if other == uuid.Nil {
			return fmt.Errorf("%w: related organization_id", ErrInvalidReference)
		}
		if other != org {
			return fmt.Errorf("%w", ErrCrossTenant)
		}
	}
	return nil
}
