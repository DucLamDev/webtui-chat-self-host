package botsecrets

import "testing"

func TestEncryptedBotSecretRoundTripAndMask(t *testing.T) {
	reference, err := Encrypt("a-production-master-secret-with-enough-entropy", "sk-customer-secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if !IsEncrypted(reference) {
		t.Fatalf("Encrypt() reference = %q, want an encrypted vault reference", reference)
	}
	if reference == "sk-customer-secret" {
		t.Fatal("Encrypt() stored plaintext")
	}
	plaintext, err := Decrypt("a-production-master-secret-with-enough-entropy", reference)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "sk-customer-secret" {
		t.Fatalf("Decrypt() = %q", plaintext)
	}
	if !IsMasked(MaskedReference()) {
		t.Fatalf("MaskedReference() = %q", MaskedReference())
	}
}
