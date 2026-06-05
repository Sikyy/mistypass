package enterprise

import (
	"testing"
	"time"
)

func TestMarkSAMLAssertionConsumed(t *testing.T) {
	svc := &Service{}
	future := time.Now().UTC().Add(5 * time.Minute)

	if !svc.markSAMLAssertionConsumed("assertion-1", future) {
		t.Fatal("first use of an assertion id must be accepted")
	}
	if svc.markSAMLAssertionConsumed("assertion-1", future) {
		t.Fatal("replay of the same assertion id must be rejected")
	}
	if !svc.markSAMLAssertionConsumed("assertion-2", future) {
		t.Fatal("a different assertion id must be accepted")
	}
	if !svc.markSAMLAssertionConsumed("", future) {
		t.Fatal("empty assertion id must not be treated as a replay")
	}

	// A past NotOnOrAfter is clamped to a default future window, so the entry is
	// still recorded and an immediate replay is rejected.
	past := time.Now().UTC().Add(-time.Hour)
	if !svc.markSAMLAssertionConsumed("assertion-3", past) {
		t.Fatal("first use must be accepted even when NotOnOrAfter is in the past")
	}
	if svc.markSAMLAssertionConsumed("assertion-3", past) {
		t.Fatal("immediate replay must be rejected")
	}
}
