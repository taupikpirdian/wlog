package ticket

import (
	"errors"
	"testing"
)

func TestKeyPatternConfigurationAndExtraction(t *testing.T) {
	if _, err := NewKeyPattern(""); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("empty pattern: %v", err)
	}
	if _, err := NewKeyPattern("["); err == nil {
		t.Fatal("accepted invalid regex")
	}
	var zero KeyPattern
	if key, found := zero.Extract("OOT-1"); key != "" || found {
		t.Fatalf("zero pattern matched %q", key)
	}
	if err := zero.Validate("OOT-1"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("zero pattern validation: %v", err)
	}
	pattern, err := NewKeyPattern(`[A-Z][A-Z0-9]+-[0-9]+`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		message, key string
		found        bool
	}{
		{"[fix] OOT-3751 fix tax\nOOT-3668 secondary", "OOT-3751", true},
		{"PROJECT1-999 support", "PROJECT1-999", true},
		{"fix without ticket", "", false},
	} {
		key, found := pattern.Extract(tc.message)
		if key != tc.key || found != tc.found {
			t.Fatalf("extract %q: %q %v", tc.message, key, found)
		}
	}
	for _, key := range []string{"", "oot-1", "OOT-1 extra", "prefix OOT-1"} {
		if err := pattern.Validate(key); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("accepted %q: %v", key, err)
		}
	}
	if err := pattern.Validate("OOT-3751"); err != nil {
		t.Fatal(err)
	}
}

func TestNewTicketNormalizesOptionalTitle(t *testing.T) {
	pattern, _ := NewKeyPattern(`OOT-[0-9]+`)
	if _, err := pattern.New("OTHER-1", "Title"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid ticket: %v", err)
	}
	for _, tc := range []struct {
		title, expected string
		absent          bool
	}{
		{" \t ", "", true}, {"", "", true}, {"  Master title  ", "Master title", false},
	} {
		value, err := pattern.New("OOT-1", tc.title)
		if err != nil || value.Key != "OOT-1" || (value.Title == nil) != tc.absent {
			t.Fatalf("ticket=%+v error=%v", value, err)
		}
		if value.Title != nil && *value.Title != tc.expected {
			t.Fatalf("title=%q", *value.Title)
		}
	}
}
