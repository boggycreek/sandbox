// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package libbp

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/libbp/resp"
)

// ClientConfig configuration parameters for connecting to the backplane
type ClientConfig struct {
	Host           string
	Port           int
	Username       string
	Password       string
	AgentID        string
	SigningKey     ed25519.PrivateKey
	SigningKeyPEM  string
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
}

// Client represents an authenticated connection to the Valkey Backplane
type Client struct {
	cfg        ClientConfig
	conn       net.Conn
	reader     *resp.Reader
	writer     *resp.Writer
	mu         sync.Mutex
	closed     bool
	pubKeys    map[string]ed25519.PublicKey
	pubKeysMu  sync.RWMutex
}

// Dial connects and authenticates to the Valkey Backplane
func Dial(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 6379
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.AgentID == "" {
		cfg.AgentID = cfg.Username
	}

	// Parse signing key if PEM provided
	if cfg.SigningKey == nil && cfg.SigningKeyPEM != "" {
		priv, err := DecodePrivateKeyPEM(cfg.SigningKeyPEM)
		if err == nil {
			cfg.SigningKey = priv
		}
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	dialer := net.Dialer{Timeout: cfg.ConnectTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to Valkey at %s: %w", addr, err)
	}

	c := &Client{
		cfg:     cfg,
		conn:    conn,
		reader:  resp.NewReader(conn),
		writer:  resp.NewWriter(conn),
		pubKeys: make(map[string]ed25519.PublicKey),
	}

	// Authenticate if password provided
	if cfg.Password != "" {
		var authVal resp.Value
		if cfg.Username != "" {
			authVal, err = c.execInternal("AUTH", cfg.Username, cfg.Password)
		} else {
			authVal, err = c.execInternal("AUTH", cfg.Password)
		}
		if err != nil {
			c.Close()
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		if authVal.Type == resp.TypeError {
			c.Close()
			return nil, fmt.Errorf("authentication error: %s", authVal.Str)
		}
	}

	return c, nil
}

// Close closes the underlying network connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Exec sends a raw command and returns the RESP value
func (c *Client) Exec(ctx context.Context, cmd string, args ...string) (resp.Value, error) {
	if err := AssertAllowed(c.cfg.AgentID, cmd, args...); err != nil {
		return resp.Value{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.execInternal(cmd, args...)
}

func (c *Client) execInternal(cmd string, args ...string) (resp.Value, error) {
	if c.closed {
		return resp.Value{}, errors.New("client connection closed")
	}
	fullCmd := append([]string{cmd}, args...)
	if err := c.writer.WriteCommand(fullCmd...); err != nil {
		return resp.Value{}, err
	}
	val, err := c.reader.ReadValue()
	if err != nil {
		return resp.Value{}, err
	}
	if val.Type == resp.TypeError {
		return val, resp.ReadError(val)
	}
	return val, nil
}

// NextSeq increments and returns the next monotonic sequence number for the agent
func (c *Client) NextSeq(ctx context.Context) (int64, error) {
	seqKey := SeqKey(c.cfg.AgentID)
	val, err := c.Exec(ctx, "INCR", seqKey)
	if err != nil {
		return 0, err
	}
	return val.AsInt()
}

// Say broadcasts a message to the fleet (<id>:out)
func (c *Client) Say(ctx context.Context, content string, blobPath string) (*Message, error) {
	seq, err := c.NextSeq(ctx)
	if err != nil {
		seq = 1
	}

	now := time.Now().UnixMilli()
	msg := &Message{
		Sender:    c.cfg.AgentID,
		Content:   content,
		BlobPath:  blobPath,
		Timestamp: now,
		Seq:       seq,
		Citation:  FormatCitation(c.cfg.AgentID, seq),
	}

	if c.cfg.SigningKey != nil {
		msg.Signature = SignPayload(c.cfg.SigningKey, msg.SigningPayload())
		msg.IsSigned = true
		msg.IsVerified = true
	}

	outStream := OutKey(c.cfg.AgentID)
	args := append([]string{outStream, "*"}, msg.ToFieldValues()...)
	val, err := c.Exec(ctx, "XADD", args...)
	if err != nil {
		return nil, fmt.Errorf("failed broadcasting message to %s: %w", outStream, err)
	}

	msgID, err := val.AsString()
	if err != nil {
		return nil, err
	}
	msg.ID = msgID
	return msg, nil
}

// Tell sends a direct point-to-point message to an agent's inbox (<recipient>:inbox)
func (c *Client) Tell(ctx context.Context, recipient string, content string) (*Message, error) {
	seq, err := c.NextSeq(ctx)
	if err != nil {
		seq = 1
	}

	now := time.Now().UnixMilli()
	msg := &Message{
		Sender:      c.cfg.AgentID,
		Destination: recipient,
		Content:     content,
		Timestamp:   now,
		Seq:         seq,
		Citation:    FormatCitation(c.cfg.AgentID, seq),
	}

	if c.cfg.SigningKey != nil {
		msg.Signature = SignPayload(c.cfg.SigningKey, msg.SigningPayload())
		msg.IsSigned = true
		msg.IsVerified = true
	}

	inboxStream := InboxKey(recipient)
	args := append([]string{inboxStream, "*"}, msg.ToFieldValues()...)
	val, err := c.Exec(ctx, "XADD", args...)
	if err != nil {
		return nil, fmt.Errorf("failed delivering message to %s: %w", inboxStream, err)
	}

	msgID, err := val.AsString()
	if err != nil {
		return nil, err
	}
	msg.ID = msgID
	return msg, nil
}

// Reply sends a threaded direct message citing a parent message ID
func (c *Client) Reply(ctx context.Context, replyToMsgID string, recipient string, content string) (*Message, error) {
	seq, err := c.NextSeq(ctx)
	if err != nil {
		seq = 1
	}

	now := time.Now().UnixMilli()
	msg := &Message{
		Sender:      c.cfg.AgentID,
		Destination: recipient,
		Content:     content,
		ReplyTo:     replyToMsgID,
		Timestamp:   now,
		Seq:         seq,
		Citation:    FormatCitation(c.cfg.AgentID, seq),
	}

	if c.cfg.SigningKey != nil {
		msg.Signature = SignPayload(c.cfg.SigningKey, msg.SigningPayload())
		msg.IsSigned = true
		msg.IsVerified = true
	}

	inboxStream := InboxKey(recipient)
	args := append([]string{inboxStream, "*"}, msg.ToFieldValues()...)
	val, err := c.Exec(ctx, "XADD", args...)
	if err != nil {
		return nil, fmt.Errorf("failed sending reply to %s: %w", inboxStream, err)
	}

	msgID, err := val.AsString()
	if err != nil {
		return nil, err
	}
	msg.ID = msgID
	return msg, nil
}

// GetCursor retrieves the last read stream ID for a given stream key
func (c *Client) GetCursor(ctx context.Context, streamKey string) (string, error) {
	cursorKey := CursorKey(c.cfg.AgentID)
	val, err := c.Exec(ctx, "HGET", cursorKey, streamKey)
	if err != nil || val.IsNull {
		return "0-0", nil
	}
	return val.AsString()
}

// SetCursor saves the read cursor for a given stream key
func (c *Client) SetCursor(ctx context.Context, streamKey string, lastID string) error {
	cursorKey := CursorKey(c.cfg.AgentID)
	_, err := c.Exec(ctx, "HSET", cursorKey, streamKey, lastID)
	return err
}

// Recv reads new messages from own inbox and all peer broadcast streams since last cursor
func (c *Client) Recv(ctx context.Context, blockSeconds int) ([]*Message, error) {
	// Discover all active streams (inbox + all *:out streams)
	myInbox := InboxKey(c.cfg.AgentID)
	streams := []string{myInbox}

	peers, _ := c.Peers(ctx)
	for _, p := range peers {
		if strings.ToLower(p.ID) != strings.ToLower(c.cfg.AgentID) {
			streams = append(streams, OutKey(p.ID))
		}
	}

	var streamArgs []string
	var cursors []string

	for _, s := range streams {
		cur, _ := c.GetCursor(ctx, s)
		if cur == "" {
			cur = "0-0"
		}
		streamArgs = append(streamArgs, s)
		cursors = append(cursors, cur)
	}

	args := []string{}
	if blockSeconds > 0 {
		args = append(args, "BLOCK", strconv.Itoa(blockSeconds*1000))
	}
	args = append(args, "STREAMS")
	args = append(args, streamArgs...)
	args = append(args, cursors...)

	val, err := c.Exec(ctx, "XREAD", args...)
	if err != nil {
		if errors.Is(err, resp.ErrNil) {
			return []*Message{}, nil
		}
		return nil, err
	}
	if val.IsNull {
		return []*Message{}, nil
	}

	return c.parseXReadResponse(ctx, val)
}

// Human returns messages from the authoritative human operator broadcast log newest-first
func (c *Client) Human(ctx context.Context, count int) ([]*Message, error) {
	if count <= 0 {
		count = 20
	}
	// Discover human name
	humanName := "human"
	nameVal, err := c.Exec(ctx, "GET", KeyHumanName)
	if err == nil && !nameVal.IsNull {
		if nameStr, err := nameVal.AsString(); err == nil && nameStr != "" {
			humanName = nameStr
		}
	}

	humanOut := OutKey(humanName)
	val, err := c.Exec(ctx, "XREVRANGE", humanOut, "+", "-", "COUNT", strconv.Itoa(count))
	if err != nil {
		return nil, err
	}

	arr, err := val.AsArray()
	if err != nil {
		return []*Message{}, nil
	}

	var messages []*Message
	for _, item := range arr {
		entry, err := item.AsArray()
		if err != nil || len(entry) < 2 {
			continue
		}
		id := entry[0].String()
		fields, err := entry[1].AsMap()
		if err != nil {
			continue
		}
		fieldMap := make(map[string]string, len(fields))
		for k, v := range fields {
			fieldMap[k] = v.String()
		}
		msg := MessageFromFields(id, fieldMap)
		c.VerifyMessage(ctx, msg)
		messages = append(messages, msg)
	}
	return messages, nil
}

// Peers returns the discovered list of active agents and their statuses
func (c *Client) Peers(ctx context.Context) ([]*Peer, error) {
	// Scan for *:out keys
	val, err := c.Exec(ctx, "SCAN", "0", "MATCH", "*:out", "COUNT", "100")
	if err != nil {
		return nil, err
	}
	arr, err := val.AsArray()
	if err != nil || len(arr) < 2 {
		return []*Peer{}, nil
	}

	keysArray, err := arr[1].AsArray()
	if err != nil {
		return []*Peer{}, nil
	}

	currentLiaison, _ := c.GetLiaison(ctx)

	var peers []*Peer
	seen := make(map[string]bool)

	for _, k := range keysArray {
		keyStr := k.String()
		if !strings.HasSuffix(keyStr, ":out") {
			continue
		}
		agentID := strings.TrimSuffix(keyStr, ":out")
		if agentID == "" || seen[agentID] {
			continue
		}
		seen[agentID] = true

		status, _ := c.GetStatus(ctx, agentID)
		peer := &Peer{
			ID:        agentID,
			Status:    status,
			IsLiaison: strings.EqualFold(agentID, currentLiaison),
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

// SetStatus sets the 300s ephemeral status message for this agent
func (c *Client) SetStatus(ctx context.Context, statusText string) error {
	statusKey := StatusKey(c.cfg.AgentID)
	_, err := c.Exec(ctx, "SET", statusKey, statusText, "EX", strconv.Itoa(StatusTTLSeconds))
	return err
}

// GetStatus retrieves the status message for a given agent
func (c *Client) GetStatus(ctx context.Context, agentID string) (string, error) {
	statusKey := StatusKey(agentID)
	val, err := c.Exec(ctx, "GET", statusKey)
	if err != nil || val.IsNull {
		return "", nil
	}
	return val.AsString()
}

// SetFinger stores profile and role metadata for this agent
func (c *Client) SetFinger(ctx context.Context, info map[string]string) error {
	fingerKey := FingerKey(c.cfg.AgentID)
	var args []string
	args = append(args, fingerKey)
	for k, v := range info {
		args = append(args, k, v)
	}
	_, err := c.Exec(ctx, "HSET", args...)
	return err
}

// GetFinger retrieves profile metadata for an agent
func (c *Client) GetFinger(ctx context.Context, agentID string) (map[string]string, error) {
	fingerKey := FingerKey(agentID)
	val, err := c.Exec(ctx, "HGETALL", fingerKey)
	if err != nil || val.IsNull {
		return nil, err
	}
	m, err := val.AsMap()
	if err != nil {
		return nil, err
	}
	res := make(map[string]string, len(m))
	for k, v := range m {
		res[k] = v.String()
	}
	return res, nil
}

// ParkBlob stores a large payload into a 24-hour TTL key (<id>:blob:<ts>)
func (c *Client) ParkBlob(ctx context.Context, data []byte) (string, error) {
	ts := time.Now().UnixMilli()
	blobKey := BlobKey(c.cfg.AgentID, ts)
	_, err := c.Exec(ctx, "SET", blobKey, string(data), "EX", strconv.Itoa(BlobTTLSeconds))
	if err != nil {
		return "", err
	}
	return blobKey, nil
}

// GetBlob retrieves a parked blob by key
func (c *Client) GetBlob(ctx context.Context, blobKey string) ([]byte, error) {
	val, err := c.Exec(ctx, "GET", blobKey)
	if err != nil || val.IsNull {
		return nil, fmt.Errorf("blob %s not found", blobKey)
	}
	str, err := val.AsString()
	if err != nil {
		return nil, err
	}
	return []byte(str), nil
}

// SetLiaison appoints an agent as the current fleet liaison
func (c *Client) SetLiaison(ctx context.Context, agentID string) error {
	_, err := c.Exec(ctx, "SET", KeyLiaisonCurrent, strings.ToLower(agentID))
	return err
}

// GetLiaison gets the current appointed liaison agent
func (c *Client) GetLiaison(ctx context.Context) (string, error) {
	val, err := c.Exec(ctx, "GET", KeyLiaisonCurrent)
	if err != nil || val.IsNull {
		return "", nil
	}
	return val.AsString()
}

// GetPollInterval reads poll-interval key from Valkey, returns default 60 if unset or invalid (valid range: 5 to 3600)
func (c *Client) GetPollInterval(ctx context.Context) (int, error) {
	val, err := c.Exec(ctx, "GET", KeyPollInterval)
	if err != nil {
		return DefaultPollInterval, err
	}
	if val.IsNull {
		return DefaultPollInterval, nil
	}
	str, err := val.AsString()
	if err != nil {
		return DefaultPollInterval, nil
	}
	secs, err := strconv.Atoi(strings.TrimSpace(str))
	if err != nil || secs < MinPollInterval || secs > MaxPollInterval {
		return DefaultPollInterval, nil
	}
	return secs, nil
}

// SetPollInterval sets the shared poll interval in Valkey (valid range: 5 to 3600)
func (c *Client) SetPollInterval(ctx context.Context, seconds int) error {
	if seconds < MinPollInterval || seconds > MaxPollInterval {
		return fmt.Errorf("poll interval must be between %d and %d seconds, got %d", MinPollInterval, MaxPollInterval, seconds)
	}
	_, err := c.Exec(ctx, "SET", KeyPollInterval, strconv.Itoa(seconds))
	return err
}

// RegisterIdentity publishes an identity attestation and public key
func (c *Client) RegisterIdentity(ctx context.Context, rec IdentityRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	idKey := IdentityKey(rec.Name)
	_, err = c.Exec(ctx, "SET", idKey, string(data))
	return err
}

// GetIdentity retrieves an identity record and caches its public key
func (c *Client) GetIdentity(ctx context.Context, id string) (*IdentityRecord, error) {
	idKey := IdentityKey(id)
	val, err := c.Exec(ctx, "GET", idKey)
	if err != nil || val.IsNull {
		return nil, fmt.Errorf("identity %s not found", id)
	}
	raw, err := val.AsString()
	if err != nil {
		return nil, err
	}
	rec, err := ParseIdentityRecord(raw)
	if err != nil {
		return nil, err
	}

	if rec.PubKey != "" {
		pub, err := DecodePublicKeyBase64(rec.PubKey)
		if err == nil {
			c.pubKeysMu.Lock()
			c.pubKeys[strings.ToLower(id)] = pub
			c.pubKeysMu.Unlock()
		}
	}
	return rec, nil
}

// VerifyMessage verifies the cryptographic signature on a message
func (c *Client) VerifyMessage(ctx context.Context, msg *Message) bool {
	if msg == nil || !msg.IsSigned || msg.Signature == "" {
		return false
	}

	sender := strings.ToLower(msg.Sender)
	c.pubKeysMu.RLock()
	pub, found := c.pubKeys[sender]
	c.pubKeysMu.RUnlock()

	if !found {
		rec, err := c.GetIdentity(ctx, sender)
		if err != nil || rec.PubKey == "" {
			return false
		}
		c.pubKeysMu.RLock()
		pub = c.pubKeys[sender]
		c.pubKeysMu.RUnlock()
	}

	if pub == nil {
		return false
	}

	valid := VerifyPayload(pub, msg.SigningPayload(), msg.Signature)
	msg.IsVerified = valid
	return valid
}

func (c *Client) parseXReadResponse(ctx context.Context, val resp.Value) ([]*Message, error) {
	streamList, err := val.AsArray()
	if err != nil {
		return nil, err
	}

	var messages []*Message

	for _, streamEntry := range streamList {
		pair, err := streamEntry.AsArray()
		if err != nil || len(pair) < 2 {
			continue
		}
		streamName := pair[0].String()
		entries, err := pair[1].AsArray()
		if err != nil {
			continue
		}

		var lastID string
		for _, entryVal := range entries {
			entryPair, err := entryVal.AsArray()
			if err != nil || len(entryPair) < 2 {
				continue
			}
			msgID := entryPair[0].String()
			fields, err := entryPair[1].AsMap()
			if err != nil {
				continue
			}

			fieldMap := make(map[string]string, len(fields))
			for k, v := range fields {
				fieldMap[k] = v.String()
			}

			msg := MessageFromFields(msgID, fieldMap)
			c.VerifyMessage(ctx, msg)
			messages = append(messages, msg)
			lastID = msgID
		}

		if lastID != "" {
			_ = c.SetCursor(ctx, streamName, lastID)
		}
	}

	return messages, nil
}

// LoadClientFromEnv builds a ClientConfig from standard backplane environment variables
func LoadClientFromEnv() ClientConfig {
	host := os.Getenv("BP_HOST")
	if host == "" {
		host = "localhost"
	}

	port := 6379
	if portStr := os.Getenv("BP_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	mode := strings.ToLower(os.Getenv("BP_MODE"))
	var username, password string
	var agentID string

	if mode == "human" {
		username = os.Getenv("HUMAN_NAME")
		if username == "" {
			username = "operator"
		}
		password = os.Getenv("HUMAN_BACKPLANE_PASSWORD")
		agentID = username
	} else {
		agentID = os.Getenv("BP_AGENT")
		if agentID == "" {
			agentID = os.Getenv("AGENT_NAME")
		}
		username = agentID
		password = os.Getenv("BP_PASSWORD")
		if password == "" {
			password = os.Getenv("AGENT_PASSWORD")
		}
	}

	signingKeyPEM := os.Getenv("BP_SIGNING_KEY_PEM")
	var signingKey ed25519.PrivateKey
	if keyPath := os.Getenv("BP_SIGNING_KEY"); keyPath != "" {
		if priv, err := LoadPrivateKeyFromFile(keyPath); err == nil {
			signingKey = priv
		}
	}

	return ClientConfig{
		Host:          host,
		Port:          port,
		Username:      username,
		Password:      password,
		AgentID:       agentID,
		SigningKey:    signingKey,
		SigningKeyPEM: signingKeyPEM,
	}
}
