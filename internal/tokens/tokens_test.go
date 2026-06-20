package tokens

import "testing"

func TestHeuristicCount(t *testing.T) {
	var h Heuristic
	if got := h.Count(""); got != 0 {
		t.Errorf("Count(empty) = %d, want 0", got)
	}
	if got := h.Count("hello"); got < 1 {
		t.Errorf("Count(hello) = %d, want >= 1", got)
	}
	// A longer string must not estimate fewer tokens than a shorter prefix.
	short := h.Count("the quick brown fox")
	long := h.Count("the quick brown fox jumps over the lazy dog repeatedly")
	if long < short {
		t.Errorf("Count not monotonic: short=%d long=%d", short, long)
	}
}
