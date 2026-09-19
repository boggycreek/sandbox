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
	BaseURL string
	Token   string
	Timeout time.Duration
}

// Client interacts with the SonarQube REST API.
type Client struct {
	baseURL    string
	token      string
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
		baseURL: baseURL,
		token:   cfg.Token,
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

func (c *Client) authenticate(req *http.Request) {
	if c.token != "" {
		req.SetBasicAuth(c.token, "")
	}
}
