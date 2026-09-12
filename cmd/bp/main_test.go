package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boggycreek/agent-sandbox/pkg/libbp"
	"github.com/boggycreek/agent-sandbox/test/harness"
)

func runCLI(args []string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestBPFullCoverage(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))
	os.Setenv("BP_MODE", "agent")
	os.Setenv("BP_AGENT", "agent-1")
	os.Setenv("BP_PASSWORD", valkey.Agent1Pass)

	// Register operator identity and KeyHumanName in Valkey
	adminClient, err := valkey.ClientFor("admin")
	if err != nil {
		t.Fatalf("admin client error: %v", err)
	}
	defer adminClient.Close()
	_, _ = adminClient.Exec(context.Background(), "SET", libbp.KeyHumanName, "operator")

	// 0. Usage & Help
	code, out, _ := runCLI([]string{})
	if code != 1 || !strings.Contains(out, "Usage: bp") {
		t.Errorf("empty args should return usage and code 1")
	}

	for _, helpArg := range []string{"help", "-h", "--help"} {
		code, out, _ = runCLI([]string{helpArg})
		if code != 0 || !strings.Contains(out, "Usage: bp") {
			t.Errorf("%s failed", helpArg)
		}
	}

	// Unknown command
	code, _, errOut := runCLI([]string{"unknowncmd"})
	if code != 1 || !strings.Contains(errOut, "unknown command") {
		t.Errorf("expected unknown command error")
	}

	// 1. Say
	// Fail on empty
	code, _, errOut = runCLI([]string{"say"})
	if code != 1 || !strings.Contains(errOut, "requires a message") {
		t.Errorf("say empty failed")
	}

	// Success say
	code, out, _ = runCLI([]string{"say", "Broadcast message from test"})
	if code != 0 || !strings.Contains(out, "Broadcast message from test") {
		t.Errorf("say failed: %s", out)
	}

	// Say with file
	tmpDir := t.TempDir()
	blobPath := filepath.Join(tmpDir, "sample.txt")
	_ = os.WriteFile(blobPath, []byte("file payload"), 0600)
	code, out, _ = runCLI([]string{"say", "--file", blobPath, "Say with attachment"})
	if code != 0 || !strings.Contains(out, "Say with attachment") {
		t.Errorf("say with file failed: %s", out)
	}

	// Say with missing file
	code, _, errOut = runCLI([]string{"say", "--file", filepath.Join(tmpDir, "missing.txt"), "msg"})
	if code != 1 || !strings.Contains(errOut, "failed reading file") {
		t.Errorf("say missing file failed")
	}

	// 2. Tell
	// Missing args
	code, _, errOut = runCLI([]string{"tell"})
	if code != 1 || !strings.Contains(errOut, "requires <agent> <message>") {
		t.Errorf("tell missing args failed")
	}

	// Success
	code, out, _ = runCLI([]string{"tell", "agent-2", "Direct task"})
	if code != 0 || !strings.Contains(out, "Direct task") {
		t.Errorf("tell failed: %s", out)
	}

	// 3. Reply
	// Missing args
	code, _, errOut = runCLI([]string{"reply", "1-0"})
	if code != 1 || !strings.Contains(errOut, "requires <msgid> <recipient> <message>") {
		t.Errorf("reply missing args failed")
	}

	// Success
	code, out, _ = runCLI([]string{"reply", "1-0", "agent-2", "In-thread reply"})
	if code != 0 || !strings.Contains(out, "In-thread reply") {
		t.Errorf("reply failed: %s", out)
	}

	// 4. Status
	// Missing args
	code, _, errOut = runCLI([]string{"status"})
	if code != 1 || !strings.Contains(errOut, "Usage: bp status set") {
		t.Errorf("status missing args failed")
	}

	// Success
	code, out, _ = runCLI([]string{"status", "set", "active testing"})
	if code != 0 || !strings.Contains(out, "status set: active testing") {
		t.Errorf("status set failed: %s", out)
	}

	// 5. Peers
	code, out, _ = runCLI([]string{"peers"})
	if code != 0 || !strings.Contains(out, "AGENT_ID") {
		t.Errorf("peers failed: %s", out)
	}

	code, out, _ = runCLI([]string{"peers", "--json"})
	if code != 0 || !strings.Contains(out, "[") {
		t.Errorf("peers json failed: %s", out)
	}

	// 6. Finger
	code, out, _ = runCLI([]string{"finger", "agent-1"})
	if code != 0 {
		t.Errorf("finger failed: %s", out)
	}

	// 7. Liaison
	os.Setenv("BP_MODE", "human")
	os.Setenv("HUMAN_NAME", "operator")
	os.Setenv("HUMAN_BACKPLANE_PASSWORD", valkey.HumanPass)

	code, out, _ = runCLI([]string{"liaison", "get"})
	if code != 0 {
		t.Errorf("liaison empty get failed")
	}

	code, out, _ = runCLI([]string{"liaison", "set", "agent-1"})
	if code != 0 || !strings.Contains(out, "Liaison appointed") {
		t.Errorf("liaison set failed: %s", out)
	}

	code, _, errOut = runCLI([]string{"liaison", "set"})
	if code != 1 || !strings.Contains(errOut, "Usage: bp liaison set") {
		t.Errorf("liaison set without arg failed")
	}

	code, out, _ = runCLI([]string{"liaison", "get"})
	if code != 0 || !strings.Contains(out, "agent-1") {
		t.Errorf("liaison get failed: %s", out)
	}

	code, _, errOut = runCLI([]string{"liaison", "invalid-sub"})
	if code != 1 || !strings.Contains(errOut, "Usage: bp liaison") {
		t.Errorf("liaison invalid sub failed")
	}

	// 8. Human Broadcast as operator
	code, out, _ = runCLI([]string{"say", "Authoritative message from operator"})
	if code != 0 {
		t.Errorf("operator say failed: %s", out)
	}

	// 9. Recv as agent-2
	os.Setenv("BP_MODE", "agent")
	os.Setenv("BP_AGENT", "agent-2")
	os.Setenv("BP_PASSWORD", valkey.Agent2Pass)
	code, out, _ = runCLI([]string{"recv"})
	if code != 0 || !strings.Contains(out, "Direct task") {
		t.Errorf("recv failed: %s", out)
	}

	code, out, _ = runCLI([]string{"recv", "--json"})
	if code != 0 || !strings.Contains(out, "[") {
		t.Errorf("recv json failed: %s", out)
	}

	// 10. Human read as agent-2
	code, out, _ = runCLI([]string{"human"})
	if code != 0 || !strings.Contains(out, "Authoritative message") {
		t.Errorf("human failed: %s", out)
	}

	code, out, _ = runCLI([]string{"human", "--json"})
	if code != 0 || !strings.Contains(out, "[") {
		t.Errorf("human json failed: %s", out)
	}
}

func TestClientConnectionFailure(t *testing.T) {
	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", "64999") // Closed port
	os.Setenv("BP_AGENT", "agent-1")
	os.Setenv("BP_PASSWORD", "bad")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"say", "fail"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "bp error:") {
		t.Errorf("expected connection failure code 1, got %d", code)
	}

	code = Run([]string{"tell", "agent-2", "fail"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected tell failure")
	}

	code = Run([]string{"reply", "1-0", "agent-2", "fail"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected reply failure")
	}

	code = Run([]string{"recv"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected recv failure")
	}

	code = Run([]string{"human"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected human failure")
	}

	code = Run([]string{"peers"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected peers failure")
	}

	code = Run([]string{"status", "set", "fail"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected status failure")
	}

	code = Run([]string{"finger", "agent-1"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected finger failure")
	}

	code = Run([]string{"liaison", "get"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected liaison failure")
	}

	code = Run([]string{"liaison", "set", "agent-1"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected liaison set failure")
	}
}
