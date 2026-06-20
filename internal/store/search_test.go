package store

import "testing"

func TestFTSMatchExpr(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello world", `"hello" "world"`},
		{"hello, world!", `"hello" "world"`},
		{"  spaced   out  ", `"spaced" "out"`},
		{"tenant_id", `"tenant" "id"`}, // underscore splits into two tokens
		{"   ", ""},
		{"!!!", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ftsMatchExpr(tc.in, false); got != tc.want {
			t.Errorf("ftsMatchExpr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	if got := ftsMatchExpr("hello world", true); got != `"hello" OR "world"` {
		t.Errorf("ftsMatchExpr any = %q, want OR-joined", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 200); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	long := truncate("abcdef", 3)
	if long != "abc…" {
		t.Errorf("truncate(abcdef,3) = %q, want abc…", long)
	}
}
