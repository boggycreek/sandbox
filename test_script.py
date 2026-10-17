import sys
content = ""
with open("backplane/pkg/libbp/client_test.go", "r") as f:
    content = f.read()

start = content.find("func TestPeersEmpty(t *testing.T) {")
if start != -1:
    content = content[:start]

new_test = """
func TestPeersEmpty(t *testing.T) {
	server, port := startMockValkeyServer(t)
	defer server.close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Dial(ctx, ClientConfig{Host: "127.0.0.1", Port: port, AgentID: "agent-1"})
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	peers, err := client.Peers(ctx)
	if err != nil {
		t.Fatalf("Peers error: %v", err)
	}
	if peers == nil {
		t.Errorf("expected non-nil empty slice, got nil")
	}
	if len(peers) != 0 {
		t.Errorf("expected len(peers) == 0, got %d", len(peers))
	}
}
"""

with open("backplane/pkg/libbp/client_test.go", "w") as f:
    f.write(content + new_test)
