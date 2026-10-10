// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boggycreek/sandbox/backplane/pkg/libbp"
	"github.com/boggycreek/sandbox/backplane/test/harness"
)

func runCLI(args []string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func runCLIWithStdin(args []string, stdin io.Reader) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(args, stdin, &stdout, &stderr)
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

	// help --ai
	code, out, _ = runCLI([]string{"help", "--ai"})
	if code != 0 || !strings.Contains(out, "Agent Execution & Protocol Guide") {
		t.Errorf("help --ai failed: %s", out)
	}

	// whoami
	code, out, _ = runCLI([]string{"whoami"})
	if code != 0 || !strings.Contains(out, "Agent ID:   agent-1") {
		t.Errorf("whoami failed: %s", out)
	}

	// profile list
	code, out, _ = runCLI([]string{"profile", "list"})
	if code != 0 || !strings.Contains(out, "Available profiles:") {
		t.Errorf("profile list failed: %s", out)
	}

	// profile list default without sub
	code, out, _ = runCLI([]string{"profile"})
	if code != 0 || !strings.Contains(out, "Available profiles:") {
		t.Errorf("profile default sub failed: %s", out)
	}

	// profile invalid sub
	code, _, errOut := runCLI([]string{"profile", "invalid"})
	if code != 1 || !strings.Contains(errOut, "unknown profile subcommand") {
		t.Errorf("profile invalid sub failed")
	}

	// --profile flag missing arg
	code, _, errOut = runCLI([]string{"--profile"})
	if code != 1 || !strings.Contains(errOut, "--profile flag requires an argument") {
		t.Errorf("--profile missing arg failed")
	}

	// --profile flag with whoami
	code, out, _ = runCLI([]string{"--profile", "default", "whoami"})
	if code != 0 || !strings.Contains(out, "Agent ID:   agent-1") {
		t.Errorf("--profile default whoami failed: %s", out)
	}

	// --profile with custom profile file
	stateTmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateTmp)
	bpProfDir := filepath.Join(stateTmp, "bp", "profiles")
	_ = os.MkdirAll(bpProfDir, 0700)
	profFile := filepath.Join(bpProfDir, "myhost.env")
	_ = os.WriteFile(profFile, []byte("BP_AGENT=myhost-agent\nBP_PASSWORD=secret\n"), 0600)

	code, out, _ = runCLI([]string{"--profile=myhost", "whoami"})
	if code != 0 || !strings.Contains(out, "Agent ID:   myhost-agent") {
		t.Errorf("--profile=myhost whoami failed: %s", out)
	}

	// profile list with profiles in dir
	code, out, _ = runCLI([]string{"profile", "list"})
	if code != 0 || !strings.Contains(out, "myhost") {
		t.Errorf("profile list with files failed: %s", out)
	}

	// Unknown command
	code, _, errOut = runCLI([]string{"unknowncmd"})
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

	// Say with stdin ('--file -')
	code, out, _ = runCLIWithStdin([]string{"say", "--file", "-", "Say with stdin payload"}, strings.NewReader("stdin stream content"))
	if code != 0 || !strings.Contains(out, "Say with stdin payload") {
		t.Errorf("say with stdin failed: %s", out)
	}

	// Say with stdin ('--file -') and nil stdin
	code, _, errOut = runCLIWithStdin([]string{"say", "--file", "-", "Say with nil stdin"}, nil)
	if code != 1 || !strings.Contains(errOut, "stdin is nil") {
		t.Errorf("say with nil stdin should fail: %s", errOut)
	}

	// Say with missing file
	code, _, errOut = runCLI([]string{"say", "--file", filepath.Join(tmpDir, "missing.txt"), "msg"})
	if code != 1 || !strings.Contains(errOut, "failed reading file") {
		t.Errorf("say missing file failed")
	}

	// Say with invalid flag
	code, _, _ = runCLI([]string{"say", "--invalid-flag"})
	if code != 1 {
		t.Errorf("say invalid flag should return 1")
	}

	// Say with missing flag argument
	code, _, errOut = runCLI([]string{"say", "--file"})
	if code != 1 || !strings.Contains(errOut, "flag needs an argument") {
		t.Errorf("say missing flag argument failed: %s", errOut)
	}

	// Say with inline file flag (--file= and -file=)
	code, out, _ = runCLI([]string{"say", "--file=" + blobPath, "Say inline flag"})
	if code != 0 || !strings.Contains(out, "Say inline flag") {
		t.Errorf("say with --file= failed: %s", out)
	}
	code, out, _ = runCLI([]string{"say", "-file=" + blobPath, "Say dash file inline flag"})
	if code != 0 || !strings.Contains(out, "Say dash file inline flag") {
		t.Errorf("say with -file= failed: %s", out)
	}

	// 2. Tell
	// Missing args
	code, _, errOut = runCLI([]string{"tell"})
	if code != 1 || !strings.Contains(errOut, "requires <agent> <message>") {
		t.Errorf("tell missing args failed")
	}

	// Tell missing args with file
	code, _, errOut = runCLI([]string{"tell", "--file", blobPath})
	if code != 1 || !strings.Contains(errOut, "requires <agent> <message>") {
		t.Errorf("tell missing recipient with file failed")
	}

	// Tell with missing flag argument
	code, _, errOut = runCLI([]string{"tell", "agent-2", "--file"})
	if code != 1 || !strings.Contains(errOut, "flag needs an argument") {
		t.Errorf("tell missing flag argument failed: %s", errOut)
	}

	// Tell with inline file flag (--file= and -file=)
	code, out, _ = runCLI([]string{"tell", "agent-2", "--file=" + blobPath})
	if code != 0 || !strings.Contains(out, "Payload attached:") {
		t.Errorf("tell with --file= failed: %s", out)
	}
	code, out, _ = runCLI([]string{"tell", "agent-2", "-file=" + blobPath})
	if code != 0 || !strings.Contains(out, "Payload attached:") {
		t.Errorf("tell with -file= failed: %s", out)
	}

	// Tell with invalid flag
	code, _, _ = runCLI([]string{"tell", "--invalid-flag"})
	if code != 1 {
		t.Errorf("tell invalid flag should return 1")
	}

	// Tell with '--' end-of-options delimiter
	code, out, _ = runCLI([]string{"tell", "agent-2", "--", "--not-a-flag"})
	if code != 0 || !strings.Contains(out, "--not-a-flag") {
		t.Errorf("tell with -- delimiter failed: %s", out)
	}

	// Success plain
	code, out, _ = runCLI([]string{"tell", "agent-2", "Direct task"})
	if code != 0 || !strings.Contains(out, "Direct task") {
		t.Errorf("tell failed: %s", out)
	}

	// Tell with file from disk (content only from file)
	code, out, _ = runCLI([]string{"tell", "agent-2", "--file", blobPath})
	if code != 0 || !strings.Contains(out, "Payload attached:") {
		t.Errorf("tell with file failed: %s", out)
	}

	// Tell with file from disk and extra message text
	code, out, _ = runCLI([]string{"tell", "agent-2", "--file", blobPath, "Prefix text"})
	if code != 0 || !strings.Contains(out, "Prefix text") || !strings.Contains(out, "Payload attached:") {
		t.Errorf("tell with file and prefix failed: %s", out)
	}

	// Tell with stdin ('--file -') alone
	code, out, _ = runCLIWithStdin([]string{"tell", "agent-2", "--file", "-"}, strings.NewReader("tell stdin body"))
	if code != 0 || !strings.Contains(out, "Payload attached:") {
		t.Errorf("tell with stdin failed: %s", out)
	}

	// Tell with stdin ('--file -') and message text
	code, out, _ = runCLIWithStdin([]string{"tell", "agent-2", "--file", "-", "Header note:"}, strings.NewReader("piped lines"))
	if code != 0 || !strings.Contains(out, "Header note:") || !strings.Contains(out, "Payload attached:") {
		t.Errorf("tell with stdin and note failed: %s", out)
	}

	// Tell with missing file
	code, _, errOut = runCLI([]string{"tell", "agent-2", "--file", filepath.Join(tmpDir, "missing.txt")})
	if code != 1 || !strings.Contains(errOut, "failed reading file") {
		t.Errorf("tell with missing file should fail")
	}

	// Tell with stdin and nil stdin
	code, _, errOut = runCLIWithStdin([]string{"tell", "agent-2", "--file", "-"}, nil)
	if code != 1 || !strings.Contains(errOut, "stdin is nil") {
		t.Errorf("tell with nil stdin should fail: %s", errOut)
	}

	// 2b. Post
	// Post missing args
	code, _, errOut = runCLI([]string{"post"})
	if code != 1 || !strings.Contains(errOut, "requires <file> [to <agent>]") {
		t.Errorf("post missing args failed: %s", errOut)
	}

	// Post --file missing argument
	code, _, errOut = runCLI([]string{"post", "--file"})
	if code != 1 || !strings.Contains(errOut, "flag --file needs an argument") {
		t.Errorf("post --file missing argument failed: %s", errOut)
	}

	// Post missing recipient after 'to'
	code, _, errOut = runCLI([]string{"post", blobPath, "to"})
	if code != 1 || !strings.Contains(errOut, "post to requires exactly <agent>") {
		t.Errorf("post to missing recipient failed: %s", errOut)
	}

	// Post unexpected arguments after 'to <agent>'
	code, _, errOut = runCLI([]string{"post", blobPath, "to", "agent-2", "extra"})
	if code != 1 || !strings.Contains(errOut, "post to requires exactly <agent>") {
		t.Errorf("post to extra args failed: %s", errOut)
	}

	// Post unexpected arguments after direct <agent>
	code, _, errOut = runCLI([]string{"post", blobPath, "agent-2", "extra"})
	if code != 1 || !strings.Contains(errOut, "post unexpected arguments after <file>") {
		t.Errorf("post direct extra args failed: %s", errOut)
	}

	// Post missing file
	code, _, errOut = runCLI([]string{"post", filepath.Join(tmpDir, "missing.txt")})
	if code != 1 || !strings.Contains(errOut, "failed reading file") {
		t.Errorf("post missing file failed: %s", errOut)
	}

	// Post empty file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	_ = os.WriteFile(emptyFile, []byte(""), 0600)
	code, _, errOut = runCLI([]string{"post", emptyFile})
	if code != 1 || !strings.Contains(errOut, "file is empty or contains only whitespace") {
		t.Errorf("post empty file failed: %s", errOut)
	}

	// Post whitespace-only file
	wsFile := filepath.Join(tmpDir, "whitespace.txt")
	_ = os.WriteFile(wsFile, []byte("  \n\t  \n  "), 0600)
	code, _, errOut = runCLI([]string{"post", wsFile})
	if code != 1 || !strings.Contains(errOut, "file is empty or contains only whitespace") {
		t.Errorf("post whitespace file failed: %s", errOut)
	}

	// Post valid file (broadcast)
	postDoc := filepath.Join(tmpDir, "post-broadcast.md")
	postContent := "# Broadcast Topic\n\nDetailed body line 1\nDetailed body line 2\n"
	_ = os.WriteFile(postDoc, []byte(postContent), 0600)
	code, out, _ = runCLI([]string{"post", postDoc})
	if code != 0 || !strings.Contains(out, "-> fleet") || !strings.Contains(out, "# Broadcast Topic") || !strings.Contains(out, "Payload attached:") {
		t.Errorf("post broadcast failed: %s", out)
	}

	// Extract blob key from output and test cat / get
	var blobKey string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Payload attached:") {
			parts := strings.Split(line, "Payload attached:")
			blobKey = strings.TrimSpace(parts[len(parts)-1])
		}
	}
	if blobKey == "" {
		t.Fatalf("failed to parse blob key from post output: %s", out)
	}

	// Cat blob retrieval
	code, catOut, _ := runCLI([]string{"cat", blobKey})
	if code != 0 || catOut != postContent {
		t.Errorf("cat blob failed: code %d, content %q", code, catOut)
	}

	// Get blob retrieval (alias for cat)
	code, getOut, _ := runCLI([]string{"get", blobKey})
	if code != 0 || getOut != postContent {
		t.Errorf("get blob failed: code %d, content %q", code, getOut)
	}

	// Cat missing argument
	code, _, errOut = runCLI([]string{"cat"})
	if code != 1 || !strings.Contains(errOut, "requires <blob-key>") {
		t.Errorf("cat missing arg failed: %s", errOut)
	}

	// Get missing argument
	code, _, errOut = runCLI([]string{"get"})
	if code != 1 || !strings.Contains(errOut, "requires <blob-key>") {
		t.Errorf("get missing arg failed: %s", errOut)
	}

	// Cat nonexistent blob key
	code, _, errOut = runCLI([]string{"cat", "agent-1:blob:nonexistent"})
	if code != 1 || !strings.Contains(errOut, "bp error:") {
		t.Errorf("cat nonexistent blob failed: %s", errOut)
	}

	// Post valid file (direct message with 'to')
	code, out, _ = runCLI([]string{"post", postDoc, "to", "agent-2"})
	if code != 0 || !strings.Contains(out, "-> agent-2") || !strings.Contains(out, "# Broadcast Topic") || !strings.Contains(out, "Payload attached:") {
		t.Errorf("post direct to agent-2 failed: %s", out)
	}

	// Post valid file (direct message without 'to')
	code, out, _ = runCLI([]string{"post", postDoc, "agent-2"})
	if code != 0 || !strings.Contains(out, "-> agent-2") || !strings.Contains(out, "# Broadcast Topic") {
		t.Errorf("post direct without 'to' failed: %s", out)
	}

	// Post with --file flag
	code, out, _ = runCLI([]string{"post", "--file", postDoc, "to", "agent-2"})
	if code != 0 || !strings.Contains(out, "-> agent-2") || !strings.Contains(out, "# Broadcast Topic") {
		t.Errorf("post --file failed: %s", out)
	}

	// Post with --file= inline flag
	code, out, _ = runCLI([]string{"post", "--file=" + postDoc})
	if code != 0 || !strings.Contains(out, "-> fleet") || !strings.Contains(out, "# Broadcast Topic") {
		t.Errorf("post --file= failed: %s", out)
	}

	// Post from stdin ('-') broadcast
	code, out, _ = runCLIWithStdin([]string{"post", "-"}, strings.NewReader("Stdin Heading Summary\n\nBody content from stdin."))
	if code != 0 || !strings.Contains(out, "-> fleet") || !strings.Contains(out, "Stdin Heading Summary") {
		t.Errorf("post stdin broadcast failed: %s", out)
	}

	// Post from stdin ('-') direct to agent
	code, out, _ = runCLIWithStdin([]string{"post", "-", "to", "agent-2"}, strings.NewReader("Direct Stdin Summary\n\nBody for agent-2."))
	if code != 0 || !strings.Contains(out, "-> agent-2") || !strings.Contains(out, "Direct Stdin Summary") {
		t.Errorf("post stdin direct failed: %s", out)
	}

	// Post from stdin with '--file -'
	code, out, _ = runCLIWithStdin([]string{"post", "--file", "-"}, strings.NewReader("Flag Stdin Summary\n\nBody."))
	if code != 0 || !strings.Contains(out, "Flag Stdin Summary") {
		t.Errorf("post --file - failed: %s", out)
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
	_, _ = adminClient.Exec(context.Background(), "HSET", "agent-1:finger", "role", "coder", "model", "qwen")
	code, out, _ = runCLI([]string{"finger", "agent-1"})
	if code != 0 || !strings.Contains(out, "coder") {
		t.Errorf("finger with profile failed: %s", out)
	}

	code, out, _ = runCLI([]string{"finger"})
	if code != 0 || !strings.Contains(out, "coder") {
		t.Errorf("finger default agent failed: %s", out)
	}

	code, out, _ = runCLI([]string{"peers", "--json"})
	if code != 0 || !strings.Contains(out, "[") {
		t.Errorf("peers json failed: %s", out)
	}

	// Status usage error
	code, _, errOut = runCLI([]string{"status", "badsub"})
	if code != 1 || !strings.Contains(errOut, "Usage: bp status") {
		t.Errorf("expected status usage error")
	}

	// 7. Liaison
	os.Setenv("BP_MODE", "human")
	os.Unsetenv("BP_AGENT")
	os.Unsetenv("BP_PASSWORD")
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

	code = Run([]string{"post", "dummy.txt"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected post failure")
	}

	code = Run([]string{"cat", "agent-1:blob:dummy"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected cat failure")
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

	// Flag parse errors
	code = Run([]string{"recv", "--invalid-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected recv invalid flag failure")
	}
	code = Run([]string{"human", "--invalid-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected human invalid flag failure")
	}
}

func TestBPDirectPayloadAndPeersScenario(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))
	os.Setenv("BP_MODE", "agent")
	os.Setenv("BP_AGENT", "agent-1")
	os.Setenv("BP_PASSWORD", valkey.Agent1Pass)

	adminClient, err := valkey.ClientFor("admin")
	if err != nil {
		t.Fatalf("failed to connect admin client: %v", err)
	}
	defer adminClient.Close()

	ctx := context.Background()
	// Set liaison to agent-1 and publish a message on its output stream
	_, err = adminClient.Exec(ctx, "SET", libbp.KeyLiaisonCurrent, "agent-1")
	if err != nil {
		t.Fatalf("failed to set liaison: %v", err)
	}
	_, err = adminClient.Exec(ctx, "XADD", "agent-1:out", "*", "sender", "agent-1", "content", "initial ping")
	if err != nil {
		t.Fatalf("failed to record out message: %v", err)
	}

	// 1. Verify peers output reflects liaison role
	code, out, errOut := runCLI([]string{"peers"})
	if code != 0 {
		t.Fatalf("peers failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "(liaison)") {
		t.Errorf("expected peers output to contain '(liaison)', got %q", out)
	}
	if !strings.Contains(out, "agent-1") {
		t.Errorf("expected peers output to contain agent-1, got %q", out)
	}

	// 2. Say with file only (empty message text)
	tmpDir := t.TempDir()
	attachmentPath := filepath.Join(tmpDir, "report.pdf")
	_ = os.WriteFile(attachmentPath, []byte("%PDF-1.4 mock report"), 0600)
	code, out, errOut = runCLI([]string{"say", "--file", attachmentPath})
	if code != 0 {
		t.Fatalf("say with file only failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "agent-1#") {
		t.Errorf("expected citation in say output, got %q", out)
	}

	// 3. Receive message with destination, unverified signature, and parked blob citation
	_, err = adminClient.Exec(ctx, "XADD", "agent-1:inbox", "*",
		"sender", "agent-2",
		"destination", "agent-1",
		"content", "file attached for review",
		"blob_path", "agent-2:blob:report.pdf",
		"signature", "invalid-signature-bytes",
		"timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()),
		"seq", "42",
	)
	if err != nil {
		t.Fatalf("failed to write inbox message: %v", err)
	}

	code, out, errOut = runCLI([]string{"recv"})
	if code != 0 {
		t.Fatalf("recv failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "-> agent-1") {
		t.Errorf("expected destination '-> agent-1' in output, got %q", out)
	}
	if !strings.Contains(out, "[UNVERIFIED]") {
		t.Errorf("expected unverified status in output, got %q", out)
	}
	if !strings.Contains(out, "Payload attached: agent-2:blob:report.pdf") {
		t.Errorf("expected attachment note in output, got %q", out)
	}

	// 4. Query finger profile of an unregistered agent
	code, out, errOut = runCLI([]string{"finger", "unregistered-agent"})
	if code != 0 {
		t.Fatalf("finger unregistered agent failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "[unregistered-agent] No finger profile registered.") {
		t.Errorf("expected no profile registered message, got %q", out)
	}
}

func TestMainExecution(t *testing.T) {
	if os.Getenv("TEST_BP_SUBPROCESS") == "1" {
		os.Args = []string{"bp", "help"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMainExecution")
	cmd.Env = append(os.Environ(), "TEST_BP_SUBPROCESS=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("main subprocess failed: %v, output: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Usage: bp") {
		t.Errorf("expected help output from main, got: %s", string(out))
	}
}

func TestReadPayload(t *testing.T) {
	// Stdin reading
	data, err := readPayload("-", strings.NewReader("hello from stdin"))
	if err != nil || string(data) != "hello from stdin" {
		t.Fatalf("unexpected result from readPayload stdin: %v, %s", err, string(data))
	}

	// Nil stdin reading
	_, err = readPayload("-", nil)
	if err == nil {
		t.Fatalf("expected error for nil stdin")
	}

	// Disk file reading
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(fpath, []byte("file content"), 0600)

	data, err = readPayload(fpath, nil)
	if err != nil || string(data) != "file content" {
		t.Fatalf("unexpected result from readPayload file: %v, %s", err, string(data))
	}

	// Nonexistent disk file reading
	_, err = readPayload(filepath.Join(tmpDir, "nonexistent.txt"), nil)
	if err == nil {
		t.Fatalf("expected error for nonexistent file")
	}
}

func TestExtractSummary(t *testing.T) {
	// Empty
	if _, err := extractSummary([]byte{}); err == nil {
		t.Errorf("expected error on empty slice")
	}

	// Whitespace only
	if _, err := extractSummary([]byte("   \n\t  \n  ")); err == nil {
		t.Errorf("expected error on whitespace only")
	}

	// Single line
	s, err := extractSummary([]byte("Simple line"))
	if err != nil || s != "Simple line" {
		t.Errorf("unexpected summary for single line: %v, %s", err, s)
	}

	// Multiline with leading empty lines
	s, err = extractSummary([]byte("\n\n  Header Title  \nSecond line"))
	if err != nil || s != "Header Title" {
		t.Errorf("unexpected summary for multiline: %v, %s", err, s)
	}

	// Long line > 256 runes
	longLine := strings.Repeat("A", 300)
	s, err = extractSummary([]byte(longLine))
	if err != nil {
		t.Fatalf("unexpected error for long line: %v", err)
	}
	if len([]rune(s)) != 256 || !strings.HasSuffix(s, "...") {
		t.Errorf("expected 256 runes ending in ..., got %d runes: %s", len([]rune(s)), s)
	}

	// Multibyte unicode line > 256 runes
	unicodeLongLine := strings.Repeat("日", 300)
	s, err = extractSummary([]byte(unicodeLongLine))
	if err != nil {
		t.Fatalf("unexpected error for unicode long line: %v", err)
	}
	if len([]rune(s)) != 256 || !strings.HasSuffix(s, "...") {
		t.Errorf("expected 256 runes ending in ..., got %d runes: %s", len([]rune(s)), s)
	}
}

func TestBPStateAndQueueSubcommands(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	queuePath := filepath.Join(tmpDir, "queue.jsonl")

	t.Setenv("AGENT_STATE_FILE", statePath)
	t.Setenv("AGENT_QUEUE_FILE", queuePath)

	// Missing args
	code, _, errOut := runCLI([]string{"state"})
	if code != 1 || !strings.Contains(errOut, "requires subcommand") {
		t.Errorf("expected error on empty state subcommand: %s", errOut)
	}

	// Unknown subcommand
	code, _, errOut = runCLI([]string{"state", "unknown"})
	if code != 1 || !strings.Contains(errOut, "unknown state subcommand") {
		t.Errorf("expected error on unknown state subcommand: %s", errOut)
	}

	// Get before file exists
	code, _, errOut = runCLI([]string{"state", "get"})
	if code != 1 || !strings.Contains(errOut, "does not exist") {
		t.Errorf("expected error getting nonexistent state: %s", errOut)
	}

	// Show before file exists
	code, _, errOut = runCLI([]string{"state", "show"})
	if code != 1 || !strings.Contains(errOut, "does not exist") {
		t.Errorf("expected error showing nonexistent state: %s", errOut)
	}

	// Set missing args
	code, _, errOut = runCLI([]string{"state", "set", "key"})
	if code != 1 || !strings.Contains(errOut, "requires <key> <value>") {
		t.Errorf("expected error on missing set value: %s", errOut)
	}

	// Set activity
	code, out, _ := runCLI([]string{"state", "set", "activity", "working on refactoring"})
	if code != 0 || !strings.Contains(out, "state.activity = working on refactoring") {
		t.Errorf("state set failed: %s", out)
	}

	// Set task
	code, out, _ = runCLI([]string{"state", "set", "current_task", "sndbx-d46.7"})
	if code != 0 || !strings.Contains(out, "state.current_task = sndbx-d46.7") {
		t.Errorf("state set task failed: %s", out)
	}

	// Get activity
	code, out, _ = runCLI([]string{"state", "get", "activity"})
	if code != 0 || strings.TrimSpace(out) != "working on refactoring" {
		t.Errorf("state get activity failed: %q", out)
	}

	// Get missing field
	code, _, errOut = runCLI([]string{"state", "get", "nonexistent"})
	if code != 1 || !strings.Contains(errOut, "not found") {
		t.Errorf("state get missing field should fail: %s", errOut)
	}

	// Get entire JSON
	code, out, _ = runCLI([]string{"state", "get"})
	if code != 0 || !strings.Contains(out, "working on refactoring") || !strings.Contains(out, "sndbx-d46.7") {
		t.Errorf("state get all failed: %s", out)
	}

	// Show
	code, out, _ = runCLI([]string{"state", "show"})
	if code != 0 || !strings.Contains(out, "working on refactoring") {
		t.Errorf("state show failed: %s", out)
	}

	// Invalid state JSON on get
	_ = os.WriteFile(statePath, []byte("invalid-json"), 0600)
	code, _, errOut = runCLI([]string{"state", "get", "activity"})
	if code != 1 || !strings.Contains(errOut, "invalid state JSON") {
		t.Errorf("expected error on invalid state JSON: %s", errOut)
	}
	_ = os.Remove(statePath)

	// Queue missing args
	code, _, errOut = runCLI([]string{"state", "queue"})
	if code != 1 || !strings.Contains(errOut, "requires subcommand") {
		t.Errorf("expected error on empty queue subcommand: %s", errOut)
	}

	// Queue unknown subcommand
	code, _, errOut = runCLI([]string{"state", "queue", "unknown"})
	if code != 1 || !strings.Contains(errOut, "unknown queue subcommand") {
		t.Errorf("expected error on unknown queue subcommand: %s", errOut)
	}

	// Queue push missing args
	code, _, errOut = runCLI([]string{"state", "queue", "push"})
	if code != 1 || !strings.Contains(errOut, "requires <directive>") {
		t.Errorf("expected error on empty queue push: %s", errOut)
	}

	// Queue pop on empty queue
	code, _, _ = runCLI([]string{"state", "queue", "pop"})
	if code != 1 {
		t.Errorf("expected code 1 on empty queue pop, got %d", code)
	}

	// Queue push item 1 and item 2
	code, out, _ = runCLI([]string{"state", "queue", "push", "directive one"})
	if code != 0 || !strings.Contains(out, "queued directive") {
		t.Errorf("queue push 1 failed: %s", out)
	}
	code, out, _ = runCLI([]string{"state", "queue", "push", "directive two"})
	if code != 0 || !strings.Contains(out, "queued directive") {
		t.Errorf("queue push 2 failed: %s", out)
	}

	// Queue list
	code, out, _ = runCLI([]string{"state", "queue", "list"})
	if code != 0 || !strings.Contains(out, "directive one") || !strings.Contains(out, "directive two") {
		t.Errorf("queue list failed: %s", out)
	}

	// Queue pop item 1
	code, out, _ = runCLI([]string{"state", "queue", "pop"})
	if code != 0 || strings.TrimSpace(out) != "directive one" {
		t.Errorf("queue pop 1 failed: %q", out)
	}

	// Queue pop item 2
	code, out, _ = runCLI([]string{"state", "queue", "pop"})
	if code != 0 || strings.TrimSpace(out) != "directive two" {
		t.Errorf("queue pop 2 failed: %q", out)
	}

	// Queue pop now empty
	code, _, _ = runCLI([]string{"state", "queue", "pop"})
	if code != 1 {
		t.Errorf("expected code 1 on empty queue pop after popping all")
	}

	// Queue clear
	code, out, _ = runCLI([]string{"state", "queue", "push", "temporary directive"})
	if code != 0 {
		t.Errorf("queue push failed")
	}
	code, out, _ = runCLI([]string{"state", "queue", "clear"})
	if code != 0 || !strings.Contains(out, "queue cleared") {
		t.Errorf("queue clear failed: %s", out)
	}
	code, out, _ = runCLI([]string{"state", "queue", "list"})
	if code != 0 || strings.TrimSpace(out) != "" {
		t.Errorf("expected empty queue list after clear: %q", out)
	}
}

func TestBPStateQueueConcurrent(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "queue.jsonl")
	t.Setenv("AGENT_QUEUE_FILE", queuePath)

	const numWorkers = 10
	const itemsPerWorker = 20
	errChan := make(chan error, numWorkers*itemsPerWorker)

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			for i := 0; i < itemsPerWorker; i++ {
				directive := fmt.Sprintf("worker-%d-task-%d", workerID, i)
				code, _, errOut := runCLI([]string{"state", "queue", "push", directive})
				if code != 0 {
					errChan <- fmt.Errorf("worker %d push failed: %s", workerID, errOut)
					return
				}
				errChan <- nil
			}
		}(w)
	}

	for i := 0; i < numWorkers*itemsPerWorker; i++ {
		if err := <-errChan; err != nil {
			t.Fatalf("concurrent push error: %v", err)
		}
	}

	poppedCount := 0
	for {
		code, out, _ := runCLI([]string{"state", "queue", "pop"})
		if code != 0 {
			break
		}
		if strings.TrimSpace(out) != "" {
			poppedCount++
		}
	}

	expectedCount := numWorkers * itemsPerWorker
	if poppedCount != expectedCount {
		t.Fatalf("expected %d popped items, got %d", expectedCount, poppedCount)
	}
}
