// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package sonar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClientConfig holds configuration parameters for the SonarQube API client.
type ClientConfig struct {
	BaseURL   string
	Token     string
	AdminUser string
	AdminPass string
	Timeout   time.Duration
}

// Client interacts with the SonarQube REST API.
type Client struct {
	baseURL    string
	token      string
	adminUser  string
	adminPass  string
	httpClient *http.Client
}

// SystemStatus represents the runtime status of the SonarQube instance.
type SystemStatus struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Status  string `json:"status"` // "UP", "STARTING", "DOWN", "RESTARTING"
}

// QualityGateStatus represents project quality gate evaluation results.
type QualityGateStatus struct {
	Status     string                 `json:"status"` // "OK", "WARN", "ERROR"
	Conditions []QualityGateCondition `json:"conditions,omitempty"`
}

// QualityGateCondition defines an individual threshold condition evaluation.
type QualityGateCondition struct {
	MetricKey      string `json:"metricKey"`
	Status         string `json:"status"` // "OK", "WARN", "ERROR"
	ActualValue    string `json:"actualValue"`
	ErrorThreshold string `json:"errorThreshold"`
}

type qualityGateResponseWrapper struct {
	ProjectStatus QualityGateStatus `json:"projectStatus"`
}

// Issue represents a static code analysis finding.
type Issue struct {
	Key       string `json:"key"`
	Rule      string `json:"rule"`
	Severity  string `json:"severity"` // "BLOCKER", "CRITICAL", "MAJOR", "MINOR", "INFO"
	Component string `json:"component"`
	Line      int    `json:"line"`
	Message   string `json:"message"`
	Type      string `json:"type"` // "BUG", "VULNERABILITY", "CODE_SMELL", "SECURITY_HOTSPOT"
	Status    string `json:"status"`
}

// IssuesResponse wraps the list of findings returned by the issues search API.
type IssuesResponse struct {
	Total  int     `json:"total"`
	Issues []Issue `json:"issues"`
}

// Measure represents a metric measure.
type Measure struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
}

type componentMeasuresWrapper struct {
	Component struct {
		Key      string    `json:"key"`
		Name     string    `json:"name"`
		Measures []Measure `json:"measures"`
	} `json:"component"`
}

type generateTokenResponse struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

// NewClient instantiates a SonarQube API client.
func NewClient(cfg ClientConfig) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:9000"
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL:   baseURL,
		token:     cfg.Token,
		adminUser: cfg.AdminUser,
		adminPass: cfg.AdminPass,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetSystemStatus queries the health and operational state of SonarQube.
func (c *Client) GetSystemStatus(ctx context.Context) (*SystemStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/system/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed creating status request: %w", err)
	}
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed executing status request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status request failed with HTTP %d: %s", resp.StatusCode, string(body))
	}

	var status SystemStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed decoding system status: %w", err)
	}
	return &status, nil
}

// GetProjectQualityGate retrieves the Quality Gate status for a project.
func (c *Client) GetProjectQualityGate(ctx context.Context, projectKey string) (*QualityGateStatus, error) {
	endpoint := fmt.Sprintf("%s/api/qualitygates/project_status?projectKey=%s", c.baseURL, url.QueryEscape(projectKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed creating quality gate request: %w", err)
	}
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed executing quality gate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("quality gate request failed with HTTP %d: %s", resp.StatusCode, string(body))
	}

	var wrapper qualityGateResponseWrapper
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("failed decoding quality gate response: %w", err)
	}
	return &wrapper.ProjectStatus, nil
}

// GetIssues retrieves static analysis issues matching the given filters.
func (c *Client) GetIssues(ctx context.Context, projectKey string, severity string, issueType string) (*IssuesResponse, error) {
	params := url.Values{}
	if projectKey != "" {
		params.Set("projectKeys", projectKey)
	}
	if severity != "" {
		params.Set("severities", severity)
	}
	if issueType != "" {
		params.Set("types", issueType)
	}

	endpoint := fmt.Sprintf("%s/api/issues/search?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed creating issues request: %w", err)
	}
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed executing issues request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("issues request failed with HTTP %d: %s", resp.StatusCode, string(body))
	}

	var issuesResp IssuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&issuesResp); err != nil {
		return nil, fmt.Errorf("failed decoding issues response: %w", err)
	}
	return &issuesResp, nil
}

// CreateProject registers a new project in SonarQube.
func (c *Client) CreateProject(ctx context.Context, projectKey, name string) error {
	params := url.Values{}
	params.Set("project", projectKey)
	params.Set("name", name)

	endpoint := fmt.Sprintf("%s/api/projects/create", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed creating project request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed executing create project request: %w", err)
	}
	defer resp.Body.Close()

	// 200 OK or 400 Bad Request if already exists
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create project failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetComponentMeasures retrieves requested metric values for a project or component.
func (c *Client) GetComponentMeasures(ctx context.Context, componentKey string, metricKeys []string) ([]Measure, error) {
	params := url.Values{}
	params.Set("component", componentKey)
	params.Set("metricKeys", strings.Join(metricKeys, ","))

	endpoint := fmt.Sprintf("%s/api/measures/component?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed creating measures request: %w", err)
	}
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed executing measures request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("measures request failed with HTTP %d: %s", resp.StatusCode, string(body))
	}

	var wrapper componentMeasuresWrapper
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("failed decoding measures response: %w", err)
	}
	return wrapper.Component.Measures, nil
}

// EnsureUser creates a user account in SonarQube if it does not already exist.
func (c *Client) EnsureUser(ctx context.Context, login, password, name, email string) error {
	params := url.Values{}
	params.Set("login", login)
	params.Set("password", password)
	if name == "" {
		name = login
	}
	params.Set("name", name)
	if email != "" {
		params.Set("email", email)
	}

	endpoint := fmt.Sprintf("%s/api/users/create", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed creating user request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed executing user create request: %w", err)
	}
	defer resp.Body.Close()

	// 200 OK or 400 Bad Request (if already exists)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("user create failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GenerateUserToken creates an analysis or user token for the specified user login.
func (c *Client) GenerateUserToken(ctx context.Context, login, tokenName string) (string, error) {
	// First revoke any existing token with the same name to prevent conflict
	_ = c.RevokeUserToken(ctx, login, tokenName)

	params := url.Values{}
	params.Set("login", login)
	params.Set("name", tokenName)

	endpoint := fmt.Sprintf("%s/api/user_tokens/generate", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed executing token generate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token generation failed with HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp generateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed decoding token response: %w", err)
	}
	return tokenResp.Token, nil
}

// RevokeUserToken revokes a user token by name.
func (c *Client) RevokeUserToken(ctx context.Context, login, tokenName string) error {
	params := url.Values{}
	params.Set("login", login)
	params.Set("name", tokenName)

	endpoint := fmt.Sprintf("%s/api/user_tokens/revoke", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed creating revoke token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed executing revoke token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("revoke token failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeactivateUser deactivates a user account in SonarQube.
func (c *Client) DeactivateUser(ctx context.Context, login string) error {
	params := url.Values{}
	params.Set("login", login)

	endpoint := fmt.Sprintf("%s/api/users/deactivate", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed creating deactivate user request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed executing deactivate user request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deactivate user failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) authenticate(req *http.Request) {
	if c.token != "" {
		req.SetBasicAuth(c.token, "")
	} else if c.adminUser != "" {
		req.SetBasicAuth(c.adminUser, c.adminPass)
	}
}
