package middleware

import (
	"regexp"
	"testing"
	"uuid"
)

var uuidv7Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewUUIDv7(t *testing.T) {
	seen := make(map[string]bool, 100)
	for range 100 {
		id := uuid.NewV7().String()
		if !uuidv7Re.MatchString(id) {
			t.Fatalf("not a UUIDv7: %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id generated: %q", id)
		}
		seen[id] = true
	}
}
