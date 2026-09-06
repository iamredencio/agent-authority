package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewOrganization(t *testing.T) {
	org, err := NewOrganization(OrganizationParams{Name: "Finance"})
	if err != nil {
		t.Fatalf("NewOrganization: %v", err)
	}
	if org.OrganizationID == uuid.Nil {
		t.Fatal("expected generated organization_id")
	}
	if org.Name != "Finance" {
		t.Fatalf("name = %q", org.Name)
	}
	if org.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at location = %v", org.CreatedAt.Location())
	}
}

func TestNewOrganizationMissingName(t *testing.T) {
	_, err := NewOrganization(OrganizationParams{Name: "   "})
	if !errors.Is(err, ErrRequiredField) {
		t.Fatalf("err = %v, want ErrRequiredField", err)
	}
}

func TestOrganizationValidateNilID(t *testing.T) {
	err := Organization{Name: "x", CreatedAt: time.Now().UTC()}.Validate()
	if !errors.Is(err, ErrRequiredField) {
		t.Fatalf("err = %v, want ErrRequiredField", err)
	}
}
