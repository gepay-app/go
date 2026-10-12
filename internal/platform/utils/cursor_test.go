package utils

import "testing"

func TestCursorRoundTrip(t *testing.T) {
	for _, id := range []string{"01a0be15-3032-76b1-9b6b-57a3ab92b0d8", "42"} {
		got, err := DecodeCursor(EncodeCursor(id))
		if err != nil {
			t.Fatalf("decode %q: %v", id, err)
		}
		if got != id {
			t.Fatalf("got %q, want %q", got, id)
		}
	}
}

func TestDecodeCursorEmptyIsFirstPage(t *testing.T) {
	got, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if got != "" {
		t.Fatalf("empty cursor should decode to \"\", got %q", got)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, s := range []string{"not-base64!", "***"} {
		if _, err := DecodeCursor(s); err == nil {
			t.Fatalf("want error for %q", s)
		}
	}
}

func TestParseLimit(t *testing.T) {
	cases := []struct {
		raw            string
		def, max, want int
	}{
		{"", 20, 100, 20},
		{"abc", 20, 100, 20},
		{"0", 20, 100, 20},
		{"-5", 20, 100, 20},
		{"50", 20, 100, 50},
		{"500", 20, 100, 100},
	}
	for _, tc := range cases {
		if got := ParseLimit(tc.raw, tc.def, tc.max); got != tc.want {
			t.Fatalf("ParseLimit(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}
