package models

import "testing"

func TestSecretRoundTrip(t *testing.T) {
	encrypted, err := EncryptSecret("p@ss-word")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "p@ss-word" {
		t.Fatal("secret was not encrypted")
	}
	plain, err := DecryptSecret(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "p@ss-word" {
		t.Fatalf("got %q", plain)
	}
	if _, err := DecryptSecret("not-base64!"); err == nil {
		t.Fatal("invalid ciphertext should fail")
	}
}
