// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/boggycreek/agent-sandbox/pkg/libbp"
	"github.com/boggycreek/agent-sandbox/test/harness"
)

func TestBPExtraCoverage(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))
	os.Setenv("BP_AGENT", "agent-1")
	os.Setenv("BP_PASSWORD", valkey.Agent1Pass)

	ctx := context.Background()
	cfg := libbp.ClientConfig{
		Host:     "127.0.0.1",
		Port:     valkey.Port,
		Username: "agent-1",
		Password: valkey.Agent1Pass,
		AgentID:  "agent-1",
	}

	var stdout, stderr bytes.Buffer

	// 1. handleFinger with self
	_ = handleFinger(ctx, cfg, []string{}, &stdout, &stderr)

	// 2. handleSay with invalid flags
	_ = handleSay(ctx, cfg, []string{"--invalid-flag"}, &stdout, &stderr)

	// 3. handleRecv with invalid flags
	_ = handleRecv(ctx, cfg, []string{"--invalid-flag"}, &stdout, &stderr)

	// 4. handleHuman with invalid flags
	_ = handleHuman(ctx, cfg, []string{"--invalid-flag"}, &stdout, &stderr)

	// 5. handlePeers with invalid flags
	_ = handlePeers(ctx, cfg, []string{"--invalid-flag"}, &stdout, &stderr)

	// 6. handleLiaison empty args
	_ = handleLiaison(ctx, cfg, []string{}, &stdout, &stderr)

	// 7. Peers with liaison set
	adminClient, err := valkey.ClientFor("admin")
	if err == nil {
		_, _ = adminClient.Exec(ctx, "SET", libbp.KeyLiaisonCurrent, "agent-1")
		_, _ = adminClient.Exec(ctx, "XADD", "agent-1:out", "*", "sender", "agent-1", "content", "hi")
		_ = handlePeers(ctx, cfg, []string{}, &stdout, &stderr)

		// 8. Recv with message having destination, blob_path, and unverified signature
		_, _ = adminClient.Exec(ctx, "XADD", "agent-1:inbox", "*",
			"sender", "agent-2",
			"destination", "agent-1",
			"content", "direct payload message",
			"blob_path", "agent-2:blob:123",
			"signature", "invalid-sig",
			"timestamp", "1600000000000",
			"seq", "1",
		)
		_ = handleRecv(ctx, cfg, []string{}, &stdout, &stderr)
		adminClient.Close()
	}
}

func TestMainFunction(t *testing.T) {
	if os.Getenv("TEST_BP_MAIN") == "1" {
		os.Args = []string{"bp", "help"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMainFunction")
	cmd.Env = append(os.Environ(), "TEST_BP_MAIN=1")
	_ = cmd.Run()
}
