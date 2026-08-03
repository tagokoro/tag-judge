package auth

import "testing"

func TestHashToken(t *testing.T) {
	token := "session-token"

	hash := HashToken(token)

	if hash == "" {
		t.Fatal("hash is empty")
	}
	if hash == token {
		t.Fatal("hash should not equal raw token")
	}
	if hash != HashToken(token) {
		t.Fatal("hash should be stable")
	}
}

func TestNewToken(t *testing.T) {
	token, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken returned error: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
}
