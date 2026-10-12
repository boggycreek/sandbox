// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package libbp

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClientContextCancellationPreCancelled(t *testing.T) {
	srv, port := startMockValkeyServer(t)
	defer srv.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before executing

	cfg := ClientConfig{
		Host:    "127.0.0.1",
		Port:    port,
		AgentID: "agent-ctx",
	}
	client, err := Dial(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed dialing mock valkey: %v", err)
	}
	defer client.Close()

	// 1. Exec with pre-cancelled context
	if _, err := client.Exec(ctx, "PING"); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on Exec, got: %v", err)
	}

	// 2. NextSeq with pre-cancelled context
	if _, err := client.NextSeq(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on NextSeq, got: %v", err)
	}

	// 3. Say with pre-cancelled context
	if _, err := client.Say(ctx, "hello", ""); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on Say, got: %v", err)
	}

	// 4. Say with blob and pre-cancelled context
	if _, err := client.Say(ctx, "hello", "blob1"); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on Say with blob, got: %v", err)
	}

	// 5. Tell with pre-cancelled context
	if _, err := client.Tell(ctx, "target", "hello"); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on Tell, got: %v", err)
	}

	// 6. TellWithBlob with pre-cancelled context
	if _, err := client.TellWithBlob(ctx, "target", "hello", "blob1"); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on TellWithBlob, got: %v", err)
	}

	// 7. Reply with pre-cancelled context
	if _, err := client.Reply(ctx, "parent-id", "target", "hello"); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on Reply, got: %v", err)
	}

	// 8. GetCursor with pre-cancelled context
	if _, err := client.GetCursor(ctx, "some-stream"); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on GetCursor, got: %v", err)
	}

	// 9. Recv with pre-cancelled context
	if _, err := client.Recv(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on Recv, got: %v", err)
	}

	// 10. Human with pre-cancelled context
	if _, err := client.Human(ctx, 5); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on Human, got: %v", err)
	}

	// 11. Peers with pre-cancelled context
	if _, err := client.Peers(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on Peers, got: %v", err)
	}

	// 12. GetStatus with pre-cancelled context
	if _, err := client.GetStatus(ctx, "target"); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on GetStatus, got: %v", err)
	}

	// 13. GetLiaison with pre-cancelled context
	if _, err := client.GetLiaison(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled on GetLiaison, got: %v", err)
	}
}

func TestClientContextDeadlineInFlight(t *testing.T) {
	// Start a silent server that accepts connections and reads, but never responds
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed starting silent listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	cfg := ClientConfig{
		Host:    "127.0.0.1",
		Port:    port,
		AgentID: "agent-silent",
	}
	client, err := Dial(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed dialing silent server: %v", err)
	}
	defer client.Close()

	// 1. Context with deadline
	ctxDeadline, cancelDeadline := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelDeadline()

	start := time.Now()
	_, err = client.Exec(ctxDeadline, "PING")
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("operation blocked too long: %v", elapsed)
	}

	// Subsequent command on broken client must fail with ErrClosed
	_, err = client.Exec(context.Background(), "PING")
	if !errors.Is(err, ErrClosed) {
		t.Errorf("expected ErrClosed on subsequent Exec after deadline expiry, got: %v", err)
	}

	// 2. In-flight cancellation on fresh client
	client2, err := Dial(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed dialing silent server for client2: %v", err)
	}
	defer client2.Close()

	ctxCancel, cancelFunc := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancelFunc()
	}()

	start = time.Now()
	_, err = client2.Exec(ctxCancel, "PING")
	elapsed = time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("cancellation unblock took too long: %v", elapsed)
	}

	// Subsequent command on client2 must also fail with ErrClosed
	_, err = client2.Exec(context.Background(), "PING")
	if !errors.Is(err, ErrClosed) {
		t.Errorf("expected ErrClosed on subsequent Exec after cancellation, got: %v", err)
	}
}

func TestClientReadTimeoutConfiguration(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed starting listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	cfg := ClientConfig{
		Host:        "127.0.0.1",
		Port:        port,
		AgentID:     "agent-timeout",
		ReadTimeout: 40 * time.Millisecond,
	}
	client, err := Dial(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed dialing silent server: %v", err)
	}
	defer client.Close()

	start := time.Now()
	_, err = client.Exec(context.Background(), "PING")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error on read timeout, got nil")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("read timeout took too long: %v", elapsed)
	}
}

func TestSayWithBlobAndReplyWithCitation(t *testing.T) {
	srv, port := startMockValkeyServer(t)
	defer srv.close()

	cfg := ClientConfig{
		Host:    "127.0.0.1",
		Port:    port,
		AgentID: "agent-sayblob",
	}
	client, err := Dial(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed dialing mock valkey: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 1. Say with blob
	msg, err := client.Say(ctx, "Broadcast with blob", "agent-sayblob:blob:123")
	if err != nil {
		t.Fatalf("Say failed: %v", err)
	}
	if msg.BlobPath != "agent-sayblob:blob:123" {
		t.Errorf("unexpected blob path: %s", msg.BlobPath)
	}

	// 2. Reply citing msg
	replyMsg, err := client.Reply(ctx, msg.Citation, "agent-other", "Replying to broadcast")
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}
	if replyMsg.ReplyTo != msg.Citation {
		t.Errorf("expected reply_to %s, got %s", msg.Citation, replyMsg.ReplyTo)
	}
}

func TestProfileVariantsAndEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "bp", "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("failed creating profiles dir: %v", err)
	}
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// 1. Human profile with quoted values
	humanProfile := filepath.Join(profilesDir, "human.env")
	content := `BP_MODE="human"
HUMAN_NAME='alice'
HUMAN_BACKPLANE_PASSWORD="secretpassword"
BP_PORT="6380"
`
	if err := os.WriteFile(humanProfile, []byte(content), 0600); err != nil {
		t.Fatalf("failed writing human profile: %v", err)
	}

	cfg, err := LoadClientWithProfile("human")
	if err != nil {
		t.Fatalf("failed loading human profile: %v", err)
	}
	if cfg.Mode != "human" || cfg.Username != "alice" || cfg.Port != 6380 || cfg.Password != "secretpassword" {
		t.Errorf("unexpected human config: %+v", cfg)
	}

	// 2. Profile with invalid signing key file path
	badKeyProfile := filepath.Join(profilesDir, "badkey.env")
	badKeyContent := `BP_AGENT=agent-bad
BP_PASSWORD=secret
BP_SIGNING_KEY=/path/does/not/exist.pem
`
	if err := os.WriteFile(badKeyProfile, []byte(badKeyContent), 0600); err != nil {
		t.Fatalf("failed writing bad key profile: %v", err)
	}
	if _, err := LoadClientWithProfile("badkey"); err == nil {
		t.Errorf("expected error loading profile with non-existent key file, got nil")
	}

	// 3. Profile with default human operator name
	humanDefaultProfile := filepath.Join(profilesDir, "humandefault.env")
	defaultContent := `BP_MODE=human
HUMAN_BACKPLANE_PASSWORD=pass
`
	if err := os.WriteFile(humanDefaultProfile, []byte(defaultContent), 0600); err != nil {
		t.Fatalf("failed writing human default profile: %v", err)
	}
	cfgDefault, err := LoadClientWithProfile("humandefault")
	if err != nil {
		t.Fatalf("failed loading human default profile: %v", err)
	}
	if cfgDefault.Username != "operator" {
		t.Errorf("expected username operator, got %s", cfgDefault.Username)
	}
}

func TestClientQueryErrorPaths(t *testing.T) {
	srv, port := startMockValkeyServer(t)
	defer srv.close()

	cfg := ClientConfig{
		Host:    "127.0.0.1",
		Port:    port,
		AgentID: "agent-errors",
	}
	client, err := Dial(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed dialing mock valkey: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// GetStatus for non-existent agent returns empty string
	status, err := client.GetStatus(ctx, "non-existent")
	if err != nil || status != "" {
		t.Errorf("expected empty status and nil error, got %q, %v", status, err)
	}

	// GetLiaison when unset returns empty string
	liaison, err := client.GetLiaison(ctx)
	if err != nil || liaison != "" {
		t.Errorf("expected empty liaison and nil error, got %q, %v", liaison, err)
	}

	// GetCursor when unset returns "0-0"
	cursor, err := client.GetCursor(ctx, "non-existent-stream")
	if err != nil || cursor != "0-0" {
		t.Errorf("expected 0-0 cursor and nil error, got %q, %v", cursor, err)
	}
}

func TestDialAuthFailures(t *testing.T) {
	srv, port := startMockValkeyServer(t)
	defer srv.close()
	srv.authFail = true

	// Dial with empty username, non-empty AgentID and password against authFail
	cfg := ClientConfig{
		Host:     "127.0.0.1",
		Port:     port,
		AgentID:  "agent-fail",
		Password: "wrongpassword",
	}
	_, err := Dial(context.Background(), cfg)
	if err == nil {
		t.Errorf("expected authentication error, got nil")
	}

	// Dial with empty AgentID and password against authFail
	cfgNoAgent := ClientConfig{
		Host:     "127.0.0.1",
		Port:     port,
		Password: "wrongpassword",
	}
	_, err = Dial(context.Background(), cfgNoAgent)
	if err == nil {
		t.Errorf("expected authentication error without agentID, got nil")
	}
}

func TestClientCloseNilConn(t *testing.T) {
	c := &Client{}
	if err := c.Close(); err != nil {
		t.Errorf("expected nil error on Close with nil conn, got: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("expected nil error on repeated Close, got: %v", err)
	}
}

func TestPublishMessageNextSeqError(t *testing.T) {
	srv, port := startMockValkeyServer(t)
	defer srv.close()

	cfg := ClientConfig{
		Host:    "127.0.0.1",
		Port:    port,
		AgentID: "agent-seqerr",
	}
	client, err := Dial(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed dialing mock valkey: %v", err)
	}
	_ = client.Close()

	_, err = client.Say(context.Background(), "msg", "")
	if err == nil {
		t.Errorf("expected error on Say after close, got nil")
	}
}

func TestClientLockCancellation(t *testing.T) {
	srv, port := startMockValkeyServer(t)
	defer srv.close()

	cfg := ClientConfig{
		Host:    "127.0.0.1",
		Port:    port,
		AgentID: "agent-lock",
	}
	client, err := Dial(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed dialing mock valkey: %v", err)
	}
	defer client.Close()

	// Acquire lock manually
	if err := client.lock(context.Background()); err != nil {
		t.Fatalf("failed acquiring lock: %v", err)
	}

	// Another caller with a short timeout context should abort lock acquisition
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	_, err = client.Exec(ctxTimeout, "PING")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded waiting for lock, got: %v", err)
	}

	client.unlock()
}
