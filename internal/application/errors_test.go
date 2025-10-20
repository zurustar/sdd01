package application

import "testing"

func TestValidationError_Error(t *testing.T) {
	var err *ValidationError
	if err.Error() != "" {
		t.Fatalf("expected empty string for nil error, got %q", err.Error())
	}

	empty := &ValidationError{}
	if got := empty.Error(); got != "validation failed" {
		t.Fatalf("expected generic message for empty error, got %q", got)
	}

	withFields := &ValidationError{FieldErrors: map[string]string{"field": "invalid"}}
	if got := withFields.Error(); got != "validation failed" {
		t.Fatalf("expected consistent message for populated error, got %q", got)
	}
}

func TestValidationError_HasErrors(t *testing.T) {
	if err := (&ValidationError{}).HasErrors(); err {
		t.Fatalf("expected HasErrors to report false for empty error")
	}

	if err := (&ValidationError{FieldErrors: map[string]string{"field": "bad"}}).HasErrors(); !err {
		t.Fatalf("expected HasErrors to report true when fields are present")
	}
}

func TestValidationError_AddAndMerge(t *testing.T) {
	base := &ValidationError{}
	base.add("first", "value")
	if got := base.FieldErrors["first"]; got != "value" {
		t.Fatalf("expected add to populate map, got %q", got)
	}

	other := &ValidationError{FieldErrors: map[string]string{"second": "another"}}
	base.merge(other)
	if got := base.FieldErrors["second"]; got != "another" {
		t.Fatalf("expected merge to copy field, got %q", got)
	}

	base.merge(nil)
	if len(base.FieldErrors) != 2 {
		t.Fatalf("expected merge with nil to leave fields unchanged")
	}
}
