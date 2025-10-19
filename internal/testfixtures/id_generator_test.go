package testfixtures

import "testing"

func TestIDGeneratorProducesSequentialIDs(t *testing.T) {
	gen := NewIDGenerator("entity")

	first := gen.Next()
	second := gen.Next()

	if first != "entity-1" || second != "entity-2" {
		t.Fatalf("unexpected identifiers: %q, %q", first, second)
	}
}

func TestIDGeneratorCanReset(t *testing.T) {
	gen := NewIDGenerator("resource")
	_ = gen.Next()
	gen.SetCounter(0)
	gen.SetPrefix("res")

	if next := gen.Next(); next != "res-1" {
		t.Fatalf("expected res-1 after reset, got %q", next)
	}
}
