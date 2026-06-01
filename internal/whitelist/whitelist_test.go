package whitelist

import (
	"testing"
)

func TestWhitelist(t *testing.T) {
	users := []int64{123, 456}
	groups := []int64{-100}

	w := NewWhitelist(users, groups)

	if !w.IsAllowed(123) {
		t.Error("expected 123 to be allowed")
	}
	if !w.IsAllowed(-100) {
		t.Error("expected -100 to be allowed")
	}
	if w.IsAllowed(999) {
		t.Error("expected 999 to be denied")
	}
}
