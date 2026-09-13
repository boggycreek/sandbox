// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package libbp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSigningKeypairAndPEM(t *testing.T) {
	priv, pub, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair error: %v", err)
	}

	// PEM roundtrip
	pemStr, err := EncodePrivateKeyPEM(priv)
	if err != nil {
		t.Fatalf("EncodePrivateKeyPEM error: %v", err)
	}
	if len(pemStr) == 0 {
		t.Fatalf("empty PEM string")
	}

	decodedPriv, err := DecodePrivateKeyPEM(pemStr)
	if err != nil {
		t.Fatalf("DecodePrivateKeyPEM error: %v", err)
	}

	// Public key base64 roundtrip
	pubB64 := EncodePublicKeyBase64(pub)
	decodedPub, err := DecodePublicKeyBase64(pubB64)
	if err != nil {
		t.Fatalf("DecodePublicKeyBase64 error: %v", err)
	}

	// Sign and verify payload
	payload := []byte("hello world backplane message")
	sig := SignPayload(decodedPriv, payload)
	if len(sig) == 0 {
		t.Fatalf("empty signature generated")
	}

	if !VerifyPayload(decodedPub, payload, sig) {
		t.Errorf("VerifyPayload failed for valid signature")
	}

	// Tampered payload verification failure
	tamperedPayload := []byte("tampered message")
	if VerifyPayload(decodedPub, tamperedPayload, sig) {
		t.Errorf("VerifyPayload unexpectedly succeeded for tampered payload")
	}

	// Invalid signature format
	if VerifyPayload(decodedPub, payload, "invalid-base64") {
		t.Errorf("VerifyPayload unexpectedly succeeded for invalid base64")
	}
}

func TestLoadPrivateKeyFromFile(t *testing.T) {
	priv, _, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair error: %v", err)
	}
	pemStr, err := EncodePrivateKeyPEM(priv)
	if err != nil {
		t.Fatalf("EncodePrivateKeyPEM error: %v", err)
	}

	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test-key.pem")
	if err := os.WriteFile(keyFile, []byte(pemStr), 0600); err != nil {
		t.Fatalf("failed writing key file: %v", err)
	}

	loadedPriv, err := LoadPrivateKeyFromFile(keyFile)
	if err != nil {
		t.Fatalf("LoadPrivateKeyFromFile error: %v", err)
	}
	if loadedPriv == nil {
		t.Fatalf("loaded private key is nil")
	}

	// Missing file error
	if _, err := LoadPrivateKeyFromFile(filepath.Join(tmpDir, "nonexistent")); err == nil {
		t.Errorf("expected error for nonexistent file, got nil")
	}

	// Invalid PEM decode errors
	if _, err := DecodePrivateKeyPEM("invalid pem block"); err == nil {
		t.Errorf("expected error decoding invalid PEM, got nil")
	}
	if _, err := DecodePublicKeyBase64("invalid"); err == nil {
		t.Errorf("expected error decoding invalid public key base64, got nil")
	}
	if _, err := DecodePublicKeyBase64("aGVsbG8="); err == nil { // valid base64 but wrong length (5 bytes instead of 32)
		t.Errorf("expected error decoding wrong length public key, got nil")
	}
}
