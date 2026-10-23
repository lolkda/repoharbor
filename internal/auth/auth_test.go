package auth

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestSessionTokensAreUnique(t *testing.T) {
	raw1, hash1, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	raw2, hash2, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if raw1 == raw2 || hash1 == hash2 {
		t.Fatal("expected unique tokens")
	}
	if HashToken(raw1) != hash1 {
		t.Fatal("hash mismatch")
	}
}
