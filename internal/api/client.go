package api

import (
	"bytes"
	"encoding/json"
	"errors"
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

// AuthError is returned when the server responds with 401 or 403.
type AuthError struct {
	StatusCode int
	Message    string
}

func (e *AuthError) Error() string {
	if e.StatusCode == 401 {
		return fmt.Sprintf("authentication required (401): %s", e.Message)
	}
	return fmt.Sprintf("access denied (403): %s", e.Message)
}

// IsAuthError returns true if err is (or wraps) an *AuthError.
func IsAuthError(err error) bool {
	var ae *AuthError
	return errors.As(err, &ae)
}

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

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		b, _ := io.ReadAll(resp.Body)
		return &AuthError{StatusCode: resp.StatusCode, Message: string(b)}
	}
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
	switch c.AuthMode {
	case AuthModeAPIKey:
		req.Header.Set("X-API-Key", c.Token)
	default:
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
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &AuthError{StatusCode: resp.StatusCode, Message: string(b)}
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}
	return json.Unmarshal(b, out)
}

func (c *Client) post(path string, body interface{}, out interface{}) error {
	resp, err := c.do("POST", path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &AuthError{StatusCode: resp.StatusCode, Message: string(b)}
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

func (c *Client) delete(path string) error {
	resp, err := c.do("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &AuthError{StatusCode: resp.StatusCode, Message: string(b)}
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
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
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &AuthError{StatusCode: resp.StatusCode, Message: string(b)}
	}
	if resp.StatusCode >= 400 {
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

// DeleteContainer removes a container. Force kills it if running; removeVolumes removes associated anon volumes.
func (c *Client) DeleteContainer(endpointID int, containerID string, force, removeVolumes bool) error {
	return c.delete(fmt.Sprintf(
		"/api/endpoints/%d/docker/containers/%s?force=%v&v=%v",
		endpointID, containerID, force, removeVolumes,
	))
}

// RecreateContainer asks Portainer to recreate a container (pull + remove + run).
func (c *Client) RecreateContainer(endpointID int, containerID string, pullImage bool) error {
	body := map[string]interface{}{"PullImage": pullImage}
	return c.post(
		fmt.Sprintf("/api/endpoints/%d/docker/containers/%s/recreate", endpointID, containerID),
		body, nil,
	)
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

func (c *Client) StackAction(stackID int, action string, endpointID int) error {
	resp, err := c.do(
		"POST",
		fmt.Sprintf("/api/stacks/%d/%s?endpointId=%d", stackID, action, endpointID),
		nil,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &AuthError{StatusCode: resp.StatusCode, Message: string(b)}
	}
	if resp.StatusCode >= 400 {
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
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &AuthError{StatusCode: resp.StatusCode, Message: string(b)}
	}
	if resp.StatusCode >= 400 {
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

// DeleteImage removes a Docker image from the endpoint.
func (c *Client) DeleteImage(endpointID int, imageID string, force bool) error {
	// Strip sha256: prefix if present for the URL
	id := imageID
	if len(id) > 7 && id[:7] == "sha256:" {
		id = id[7:]
	}
	return c.delete(fmt.Sprintf(
		"/api/endpoints/%d/docker/images/%s?force=%v",
		endpointID, id, force,
	))
}

// PullImage triggers a docker pull on the endpoint for the given image reference.
// The Portainer API proxies this to the Docker daemon's /images/create endpoint.
func (c *Client) PullImage(endpointID int, image string) error {
	resp, err := c.do(
		"POST",
		fmt.Sprintf("/api/endpoints/%d/docker/images/create?fromImage=%s", endpointID, image),
		nil,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("pull failed (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}

// PruneImagesReport holds the result of an image prune.
type PruneImagesReport struct {
	SpaceReclaimed int64 `json:"SpaceReclaimed"`
}

// PruneImages removes all dangling (unused, untagged) images.
func (c *Client) PruneImages(endpointID int) (*PruneImagesReport, error) {
	resp, err := c.do(
		"POST",
		fmt.Sprintf("/api/endpoints/%d/docker/images/prune?filters={\"dangling\":[\"true\"]}", endpointID),
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("prune failed (%d): %s", resp.StatusCode, string(b))
	}
	var report PruneImagesReport
	_ = json.Unmarshal(b, &report)
	return &report, nil
}

// PruneVolumesReport holds the result of a volume prune.
type PruneVolumesReport struct {
	SpaceReclaimed int64 `json:"SpaceReclaimed"`
}

// PruneVolumes removes all unused volumes.
func (c *Client) PruneVolumes(endpointID int) (*PruneVolumesReport, error) {
	resp, err := c.do(
		"POST",
		fmt.Sprintf("/api/endpoints/%d/docker/volumes/prune", endpointID),
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("prune failed (%d): %s", resp.StatusCode, string(b))
	}
	var report PruneVolumesReport
	_ = json.Unmarshal(b, &report)
	return &report, nil
}

// ListStackContainers returns all containers belonging to a stack (by its compose project label).
func (c *Client) ListStackContainers(endpointID int, stackName string) ([]Container, error) {
	var result []Container
	return result, c.get(
		fmt.Sprintf(
			"/api/endpoints/%d/docker/containers/json?all=true&filters={\"label\":[\"com.docker.compose.project=%s\"]}",
			endpointID, stackName,
		),
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

// CreateVolumeRequest holds parameters for creating a new Docker volume.
type CreateVolumeRequest struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Labels     map[string]string `json:"Labels,omitempty"`
	DriverOpts map[string]string `json:"DriverOpts,omitempty"`
}

func (c *Client) CreateVolume(endpointID int, req CreateVolumeRequest) error {
	return c.post(
		fmt.Sprintf("/api/endpoints/%d/docker/volumes/create", endpointID),
		req, nil,
	)
}

// DeleteVolume removes a named volume. Set force=true to remove even if in use (Docker may still reject).
func (c *Client) DeleteVolume(endpointID int, volumeName string, force bool) error {
	return c.delete(fmt.Sprintf(
		"/api/endpoints/%d/docker/volumes/%s?force=%v",
		endpointID, volumeName, force,
	))
}

// ─── Networks ─────────────────────────────────────────────────────────────────

type Network struct {
	ID         string            `json:"Id"`
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Scope      string            `json:"Scope"`
	Internal   bool              `json:"Internal"`
	Attachable bool              `json:"Attachable"`
	Labels     map[string]string `json:"Labels"`
	IPAM       NetworkIPAM       `json:"IPAM"`
}

type NetworkIPAM struct {
	Driver string            `json:"Driver"`
	Config []NetworkIPAMConf `json:"Config"`
}

type NetworkIPAMConf struct {
	Subnet  string `json:"Subnet,omitempty"`
	Gateway string `json:"Gateway,omitempty"`
}

type CreateNetworkRequest struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Internal   bool              `json:"Internal"`
	Attachable bool              `json:"Attachable"`
	Labels     map[string]string `json:"Labels,omitempty"`
	IPAM       *NetworkIPAM      `json:"IPAM,omitempty"`
}

func (c *Client) ListNetworks(endpointID int) ([]Network, error) {
	var result []Network
	return result, c.get(
		fmt.Sprintf("/api/endpoints/%d/docker/networks", endpointID),
		&result,
	)
}

func (c *Client) CreateNetwork(endpointID int, req CreateNetworkRequest) error {
	return c.post(
		fmt.Sprintf("/api/endpoints/%d/docker/networks/create", endpointID),
		req, nil,
	)
}

func (c *Client) DeleteNetwork(endpointID int, networkID string) error {
	return c.delete(fmt.Sprintf(
		"/api/endpoints/%d/docker/networks/%s",
		endpointID, networkID,
	))
}

// ─── Open Portainer in browser ────────────────────────────────────────────────

func (c *Client) OpenURL() string {
	return c.BaseURL
}
