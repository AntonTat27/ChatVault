package crypto

import "testing"

// testKey is a throwaway 32-byte (64 hex char) key, not used outside tests.
const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c, err := NewCipher(testKey)
	if err != nil {
		t.Fatalf("NewCipher failed: %v", err)
	}

	plaintext := "secret_notion_access_token"
	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if string(ciphertext) == plaintext {
		t.Fatalf("ciphertext must not equal plaintext")
	}

	decrypted, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestNewCipher_RejectsWrongKeyLength(t *testing.T) {
	if _, err := NewCipher("deadbeef"); err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDecrypt_RejectsTamperedCiphertext(t *testing.T) {
	c, err := NewCipher(testKey)
	if err != nil {
		t.Fatalf("NewCipher failed: %v", err)
	}
	ciphertext, err := c.Encrypt("hello")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 0xFF
	if _, err := c.Decrypt(ciphertext); err == nil {
		t.Fatal("expected decrypt to fail on tampered ciphertext")
	}
}
