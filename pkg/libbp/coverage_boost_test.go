// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package libbp

import (
	"context"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/libbp/resp"
)

func TestClientExtraCoverage(t *testing.T) {
	srv, port := startMockValkeyServer(t)
	defer srv.close()

	priv, pub, _ := GenerateKeypair()
	pemStr, _ := EncodePrivateKeyPEM(priv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ClientConfig{
		Host:          "127.0.0.1",
		Port:          port,
		Username:      "agent-1",
		Password:      "test-pw",
		AgentID:       "agent-1",
		SigningKey:    priv,
		SigningKeyPEM: pemStr,
	}

	client, err := Dial(ctx, cfg)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	// 1. NextSeq error scenario (simulate closed client)
	cClosed := &Client{closed: true, cfg: cfg}
	if _, err := cClosed.NextSeq(ctx); err == nil {
		t.Errorf("expected error on NextSeq with closed client")
	}

	// 2. Say with fallback seq (simulated by failing nextSeq or closed)
	if _, err := cClosed.Say(ctx, "content", ""); err == nil {
		t.Errorf("expected error on Say with closed client")
	}

	// 3. Tell with closed client
	if _, err := cClosed.Tell(ctx, "agent-2", "content"); err == nil {
		t.Errorf("expected error on Tell with closed client")
	}

	// 4. Reply with closed client
	if _, err := cClosed.Reply(ctx, "100-0", "agent-2", "content"); err == nil {
		t.Errorf("expected error on Reply with closed client")
	}

	// 5. Recv with nil value from server (simulate error / empty return)
	if _, err := cClosed.Recv(ctx, 0); err == nil {
		t.Errorf("expected error on Recv with closed client")
	}

	// 6. Human with closed client
	if _, err := cClosed.Human(ctx, 0); err == nil {
		t.Errorf("expected error on Human with closed client")
	}

	// 7. Peers with closed client
	if _, err := cClosed.Peers(ctx); err == nil {
		t.Errorf("expected error on Peers with closed client")
	}

	// 8. SetStatus / GetStatus / SetFinger / GetFinger with closed client
	if err := cClosed.SetStatus(ctx, "test"); err == nil {
		t.Errorf("expected error on SetStatus with closed client")
	}
	if err := cClosed.SetFinger(ctx, map[string]string{"k": "v"}); err == nil {
		t.Errorf("expected error on SetFinger with closed client")
	}
	if _, err := cClosed.GetFinger(ctx, "agent-1"); err == nil {
		t.Errorf("expected error on GetFinger with closed client")
	}

	// 9. ParkBlob with closed client
	if _, err := cClosed.ParkBlob(ctx, []byte("data")); err == nil {
		t.Errorf("expected error on ParkBlob with closed client")
	}

	// 10. SetLiaison / RegisterIdentity with closed client
	if err := cClosed.SetLiaison(ctx, "agent-1"); err == nil {
		t.Errorf("expected error on SetLiaison with closed client")
	}
	if err := cClosed.RegisterIdentity(ctx, IdentityRecord{Name: "agent-1"}); err == nil {
		t.Errorf("expected error on RegisterIdentity with closed client")
	}

	// 11. parseXReadResponse error paths
	if _, err := client.parseXReadResponse(ctx, resp.Value{Type: resp.TypeError, Str: "ERR"}); err == nil {
		t.Errorf("expected error parsing error value in parseXReadResponse")
	}

	// 12. VerifyMessage with cached public key
	pubB64 := EncodePublicKeyBase64(pub)
	_ = client.RegisterIdentity(ctx, IdentityRecord{Name: "agent-1", PubKey: pubB64})
	msg := &Message{
		Sender:    "agent-1",
		Content:   "signed message",
		Timestamp: time.Now().UnixMilli(),
		Seq:       1,
		IsSigned:  true,
	}
	msg.Signature = SignPayload(priv, msg.SigningPayload())
	if !client.VerifyMessage(ctx, msg) {
		t.Errorf("expected VerifyMessage to succeed")
	}

	// Corrupt signature
	msg.Signature = "AAAA"
	if client.VerifyMessage(ctx, msg) {
		t.Errorf("expected VerifyMessage to fail on bad signature")
	}

	// 13. Test LoadClientFromEnv edge cases
	os.Setenv("BP_MODE", "agent")
	os.Setenv("BP_PORT", "invalid-port")
	os.Setenv("BP_AGENT", "")
	os.Setenv("AGENT_NAME", "agent-fallback")
	os.Setenv("BP_PASSWORD", "")
	os.Setenv("AGENT_PASSWORD", "pw-fallback")
	cfgEnv := LoadClientFromEnv()
	if cfgEnv.AgentID != "agent-fallback" || cfgEnv.Password != "pw-fallback" || cfgEnv.Port != 6379 {
		t.Errorf("unexpected LoadClientFromEnv fallback: %+v", cfgEnv)
	}

	// PEM key directly in BP_SIGNING_KEY_PEM in Dial
	clientWithPEM, err := Dial(ctx, ClientConfig{
		Host:          "127.0.0.1",
		Port:          port,
		Username:      "agent-1",
		SigningKeyPEM: pemStr,
	})
	if err != nil {
		t.Fatalf("Dial with SigningKeyPEM failed: %v", err)
	}
	if clientWithPEM.cfg.SigningKey == nil {
		t.Errorf("expected SigningKey to be populated from SigningKeyPEM")
	}
	clientWithPEM.Close()
}

func TestSigningExtraCoverage(t *testing.T) {
	// Test PKCS8 error on decode
	_, err := DecodePrivateKeyPEM("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----")
	if err == nil {
		t.Errorf("expected error on malformed PEM payload")
	}

	// Test VerifyPayload with bad key length
	badPub := ed25519.PublicKey([]byte("short"))
	if VerifyPayload(badPub, []byte("data"), "sig") {
		t.Errorf("expected false for bad public key")
	}

	// Test LoadPrivateKeyFromFile read error
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.pem")
	_ = os.WriteFile(badFile, []byte("not a valid pem"), 0600)
	if _, err := LoadPrivateKeyFromFile(badFile); err == nil {
		t.Errorf("expected error reading invalid pem file")
	}
}

func TestRespWriterAndReaderEdgeCases(t *testing.T) {
	// EncodeCommand
	cmdBytes := resp.EncodeCommand("PING", "arg")
	if len(cmdBytes) == 0 {
		t.Errorf("EncodeCommand produced empty bytes")
	}
}
