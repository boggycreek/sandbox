// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClientConfig holds connection and authentication details for Gitea API
type ClientConfig struct {
	BaseURL   string
	AdminUser string
	AdminPass string
	Token     string
	Timeout   time.Duration
}

// Client provides an HTTP client for Gitea administrative and organizational operations
type Client struct {
	cfg        ClientConfig
	httpClient *http.Client
}

// NewClient constructs a new Gitea API client
func NewClient(cfg ClientConfig) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://127.0.0.1:3000"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.AdminUser == "" {
		cfg.AdminUser = "giteaadmin"
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// doRequest performs an authenticated HTTP request against Gitea REST API
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, []byte, error) {
	url := fmt.Sprintf("%s/api/v1%s", c.cfg.BaseURL, endpoint)

	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed creating http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.cfg.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", c.cfg.Token))
	} else if c.cfg.AdminUser != "" && c.cfg.AdminPass != "" {
		req.SetBasicAuth(c.cfg.AdminUser, c.cfg.AdminPass)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("failed reading response body: %w", err)
	}

	return resp, respBody, nil
}

// EnsureUser creates a user in Gitea if it does not already exist
func (c *Client) EnsureUser(ctx context.Context, username, password, email string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if email == "" {
		email = fmt.Sprintf("%s@local.sndbx", username)
	}

	// Check if user exists
	resp, _, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/users/%s", username), nil)
	if err == nil && resp.StatusCode == http.StatusOK {
		return nil // User already exists
	}

	// Create user via admin API
	payload := map[string]interface{}{
		"username":             username,
		"password":             password,
		"email":                email,
		"must_change_password": false,
		"send_notify":          false,
	}

	resp, respBody, err := c.doRequest(ctx, http.MethodPost, "/admin/users", payload)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("failed creating gitea user %s (status %d): %s", username, resp.StatusCode, string(respBody))
	}

	return nil
}

// DeleteUser purges a user account and associated repositories from Gitea
func (c *Client) DeleteUser(ctx context.Context, username string, purge bool) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	endpoint := fmt.Sprintf("/admin/users/%s?purge=%t", username, purge)
	resp, respBody, err := c.doRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("failed deleting gitea user %s (status %d): %s", username, resp.StatusCode, string(respBody))
	}

	return nil
}

// AddUserSSHKey associates an SSH public key with a user account
func (c *Client) AddUserSSHKey(ctx context.Context, username, title, keyContent string) error {
	username = strings.TrimSpace(username)
	keyContent = strings.TrimSpace(keyContent)
	if username == "" || keyContent == "" {
		return fmt.Errorf("username and keyContent must not be empty")
	}
	if title == "" {
		title = fmt.Sprintf("%s-key", username)
	}

	payload := map[string]interface{}{
		"title":     title,
		"key":       keyContent,
		"read_only": false,
	}

	endpoint := fmt.Sprintf("/admin/users/%s/keys", username)
	resp, respBody, err := c.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return err
	}
	// Gitea returns 201 Created on success, 422/409 if key already exists
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusUnprocessableEntity {
		return fmt.Errorf("failed adding ssh key for %s (status %d): %s", username, resp.StatusCode, string(respBody))
	}

	return nil
}

// EnsureOrg creates an organization if it does not already exist
func (c *Client) EnsureOrg(ctx context.Context, orgName string) error {
	orgName = strings.TrimSpace(orgName)
	if orgName == "" {
		return fmt.Errorf("organization name cannot be empty")
	}

	// Check if org exists
	resp, _, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/orgs/%s", orgName), nil)
	if err == nil && resp.StatusCode == http.StatusOK {
		return nil
	}

	payload := map[string]interface{}{
		"username":   orgName,
		"visibility": "public",
	}

	resp, respBody, err := c.doRequest(ctx, http.MethodPost, "/orgs", payload)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("failed creating organization %s (status %d): %s", orgName, resp.StatusCode, string(respBody))
	}

	return nil
}

// AddOrgMember adds a user to an organization
func (c *Client) AddOrgMember(ctx context.Context, orgName, username string) error {
	orgName = strings.TrimSpace(orgName)
	username = strings.TrimSpace(username)
	if orgName == "" || username == "" {
		return fmt.Errorf("orgName and username cannot be empty")
	}

	endpoint := fmt.Sprintf("/orgs/%s/members/%s", orgName, username)
	resp, respBody, err := c.doRequest(ctx, http.MethodPut, endpoint, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed adding member %s to org %s (status %d): %s", username, orgName, resp.StatusCode, string(respBody))
	}

	return nil
}

// EnsureRepo creates a repository under an organization or user namespace if missing
func (c *Client) EnsureRepo(ctx context.Context, orgName, repoName, description string, autoInit bool) error {
	orgName = strings.TrimSpace(orgName)
	repoName = strings.TrimSpace(repoName)
	if orgName == "" || repoName == "" {
		return fmt.Errorf("orgName and repoName cannot be empty")
	}

	// Check if repo exists
	resp, _, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s", orgName, repoName), nil)
	if err == nil && resp.StatusCode == http.StatusOK {
		return nil
	}

	payload := map[string]interface{}{
		"name":        repoName,
		"description": description,
		"private":     false,
		"auto_init":   autoInit,
	}

	endpoint := fmt.Sprintf("/orgs/%s/repos", orgName)
	resp, respBody, err := c.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("failed creating repository %s/%s (status %d): %s", orgName, repoName, resp.StatusCode, string(respBody))
	}

	return nil
}
