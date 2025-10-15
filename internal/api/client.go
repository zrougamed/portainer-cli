package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AuthMode controls which authentication header is sent.
// Portainer rejects requests that send both at the same time (403).
type AuthMode int

const (
	AuthModeJWT    AuthMode = iota // Authorization: Bearer <token>
	AuthModeAPIKey                 // X-API-Key: <key>
)

// Client is the Portainer API client
type Client struct {
	BaseURL    string
	Token      string
	AuthMode   AuthMode
	HTTPClient *http.Client
}

// NewClient creates a client that authenticates via JWT Bearer token.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:  baseURL,
		Token:    token,
		AuthMode: AuthModeJWT,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// NewClientWithAPIKey creates a client that authenticates via X-API-Key header.
func NewClientWithAPIKey(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL:  baseURL,
		Token:    apiKey,
		AuthMode: AuthModeAPIKey,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Authenticate logs in and stores the JWT token
func (c *Client) Authenticate(username, password string) error {
	body := map[string]string{"username": username, "password": password}
	data, _ := json.Marshal(body)

	resp, err := c.HTTPClient.Post(
		c.BaseURL+"/api/auth",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("authentication failed: status %d", resp.StatusCode)
	}

	var result struct {
		JWT string `json:"jwt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding auth response: %w", err)
	}
	c.Token = result.JWT
	return nil
}

func (c *Client) do(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// Portainer rejects requests that send BOTH headers simultaneously (403).
	// Send only the header that matches the configured auth mode.
	switch c.AuthMode {
	case AuthModeAPIKey:
		req.Header.Set("X-API-Key", c.Token)
	default: // AuthModeJWT
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	return c.HTTPClient.Do(req)
}

func (c *Client) get(path string, out interface{}) error {
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) post(path string, body interface{}, out interface{}) error {
	resp, err := c.do("POST", path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// ─── Endpoints / Environments ────────────────────────────────────────────────

type Endpoint struct {
	ID     int    `json:"Id"`
	Name   string `json:"Name"`
	URL    string `json:"URL"`
	Status int    `json:"Status"` // 1=up, 2=down
	Type   int    `json:"Type"`
}

func (c *Client) ListEndpoints() ([]Endpoint, error) {
	var result []Endpoint
	return result, c.get("/api/endpoints", &result)
}

// ─── Containers ───────────────────────────────────────────────────────────────

type Container struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
}

func (c *Client) ListContainers(endpointID int, all bool) ([]Container, error) {
	param := "false"
	if all {
		param = "true"
	}
	var result []Container
	return result, c.get(
		fmt.Sprintf("/api/endpoints/%d/docker/containers/json?all=%s", endpointID, param),
		&result,
	)
}

func (c *Client) ContainerAction(endpointID int, containerID, action string) error {
	resp, err := c.do(
		"POST",
		fmt.Sprintf("/api/endpoints/%d/docker/containers/%s/%s", endpointID, containerID, action),
		nil,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("action %s failed: %s", action, string(b))
	}
	return nil
}

func (c *Client) ContainerLogs(endpointID int, containerID string, tail int) (string, error) {
	resp, err := c.do(
		"GET",
		fmt.Sprintf("/api/endpoints/%d/docker/containers/%s/logs?stdout=true&stderr=true&tail=%d&timestamps=true",
			endpointID, containerID, tail),
		nil,
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// ─── Stacks ───────────────────────────────────────────────────────────────────

type Stack struct {
	ID         int    `json:"Id"`
	Name       string `json:"Name"`
	Type       int    `json:"Type"` // 1=swarm, 2=compose
	EndpointID int    `json:"EndpointId"`
	Status     int    `json:"Status"` // 1=active, 2=inactive
	Env        []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"Env"`
}

func (c *Client) ListStacks() ([]Stack, error) {
	var result []Stack
	return result, c.get("/api/stacks", &result)
}

func (c *Client) DeployStack(endpointID int, name, composeContent string, env map[string]string) (*Stack, error) {
	envVars := []map[string]string{}
	for k, v := range env {
		envVars = append(envVars, map[string]string{"name": k, "value": v})
	}
	body := map[string]interface{}{
		"Name":             name,
		"StackFileContent": composeContent,
		"Env":              envVars,
	}
	var result Stack
	return &result, c.post(
		fmt.Sprintf("/api/stacks/create/standalone/string?endpointId=%d", endpointID),
		body,
		&result,
	)
}

func (c *Client) StackAction(stackID int, action string) error {
	resp, err := c.do(
		"POST",
		fmt.Sprintf("/api/stacks/%d/%s", stackID, action),
		nil,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stack action failed: %s", string(b))
	}
	return nil
}

func (c *Client) DeleteStack(stackID, endpointID int) error {
	resp, err := c.do(
		"DELETE",
		fmt.Sprintf("/api/stacks/%d?endpointId=%d", stackID, endpointID),
		nil,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete stack failed: %s", string(b))
	}
	return nil
}

// ─── Images ───────────────────────────────────────────────────────────────────

type Image struct {
	ID         string   `json:"Id"`
	RepoTags   []string `json:"RepoTags"`
	Size       int64    `json:"Size"`
	Created    int64    `json:"Created"`
	Containers int64    `json:"Containers"`
}

func (c *Client) ListImages(endpointID int) ([]Image, error) {
	var result []Image
	return result, c.get(
		fmt.Sprintf("/api/endpoints/%d/docker/images/json?all=false", endpointID),
		&result,
	)
}

// ─── Volumes ──────────────────────────────────────────────────────────────────

type Volume struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Mountpoint string            `json:"Mountpoint"`
	Labels     map[string]string `json:"Labels"`
	Scope      string            `json:"Scope"`
}

type VolumesResponse struct {
	Volumes  []Volume `json:"Volumes"`
	Warnings []string `json:"Warnings"`
}

func (c *Client) ListVolumes(endpointID int) ([]Volume, error) {
	var result VolumesResponse
	err := c.get(fmt.Sprintf("/api/endpoints/%d/docker/volumes", endpointID), &result)
	return result.Volumes, err
}

// ─── Open Portainer in browser ────────────────────────────────────────────────

func (c *Client) OpenURL() string {
	return c.BaseURL
}
