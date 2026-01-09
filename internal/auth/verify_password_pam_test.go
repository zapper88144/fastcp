//go:build pam

package auth

import (
	"testing"
)

// TestVerifyPassword_Pam_NonexistentUser ensures the PAM-backed verifyPassword
// function behaves safely when a user does not exist (returns false, no panic).
func TestVerifyPassword_Pam_NonexistentUser(t *testing.T) {
	// Choose a username extremely unlikely to exist
	ok := verifyPassword("__fastcp_test_nonexistent_user__", "wrongpass")
	if ok {
		t.Fatalf("expected verifyPassword to return false for nonexistent user, got true")
	}
}
