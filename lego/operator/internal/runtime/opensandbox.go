// Package runtime is a client for the OpenSandbox Lifecycle API (the Docker-runtime
// server). Mirrors the MVP's opensandbox.js: create a sandbox from an OCI image,
// reach it via its per-sandbox endpoint, and pause/resume (real snapshots).
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Target is how the edge reaches a sandbox: host:port plus a path prefix
// (e.g. /proxy/3000 for OpenSandbox's per-sandbox endpoint).
type Target struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Prefix string `json:"prefix"`
}

func (t Target) URL() string {
	return fmt.Sprintf("http://%s:%d%s", t.Host, t.Port, t.Prefix)
}

// OpenSandbox is a client for one OpenSandbox server.
type OpenSandbox struct {
	BaseURL    string
	CPU        string
	Memory     string
	TimeoutSec int
	HTTP       *http.Client
}

func New(baseURL string) *OpenSandbox {
	return &OpenSandbox{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		CPU:        "1",
		Memory:     "512Mi",
		TimeoutSec: 86400,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
	}
}

var (
	runningStates = []string{"Running", "running", "Ready", "ready"}
	pausedStates  = []string{"Paused", "paused", "Stopped", "stopped", "Suspended"}
	failedStates  = []string{"Failed", "failed", "Error", "error"}
)

type sandbox struct {
	ID     string `json:"id"`
	Status struct {
		State   string `json:"state"`
		Message string `json:"message"`
	} `json:"status"`
}

func (o *OpenSandbox) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, o.BaseURL+path, r)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode, nil
}

// imageEntrypoint derives the entrypoint OpenSandbox requires from the built image
// (CNB → ["/cnb/process/web"]; Dockerfile → its ENTRYPOINT+CMD).
func imageEntrypoint(ctx context.Context, image string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format",
		"{{json .Config.Entrypoint}}|{{json .Config.Cmd}}", image).Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect %s: %w", image, err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
	var ep, cmd []string
	_ = json.Unmarshal([]byte(parts[0]), &ep)
	if len(parts) == 2 {
		_ = json.Unmarshal([]byte(parts[1]), &cmd)
	}
	entry := append(ep, cmd...)
	if len(entry) == 0 {
		return nil, fmt.Errorf("image %s declares no entrypoint/cmd", image)
	}
	return entry, nil
}

// Create starts a sandbox from image and returns its id once Running.
func (o *OpenSandbox) Create(ctx context.Context, image string, port int, env map[string]string, serviceID string) (string, error) {
	entry, err := imageEntrypoint(ctx, image)
	if err != nil {
		return "", err
	}
	if env == nil {
		env = map[string]string{}
	}
	env["PORT"] = strconv.Itoa(port)
	body := map[string]any{
		"image":          map[string]string{"uri": image},
		"entrypoint":     entry,
		"env":            env,
		"resourceLimits": map[string]string{"cpu": o.CPU, "memory": o.Memory},
		"metadata":       map[string]string{"bex.service": serviceID},
		"timeout":        o.TimeoutSec,
	}
	data, code, err := o.do(ctx, http.MethodPost, "/sandboxes", body)
	if err != nil {
		return "", err
	}
	if code >= 300 {
		return "", fmt.Errorf("create failed (%d): %s", code, string(data))
	}
	var s sandbox
	if err := json.Unmarshal(data, &s); err != nil || s.ID == "" {
		return "", fmt.Errorf("create: bad response: %s", string(data))
	}
	if err := o.WaitState(ctx, s.ID, runningStates, 180*time.Second); err != nil {
		return s.ID, err
	}
	return s.ID, nil
}

// WaitState polls until the sandbox reaches one of want (or fails / times out).
func (o *OpenSandbox) WaitState(ctx context.Context, id string, want []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, _, err := o.do(ctx, http.MethodGet, "/sandboxes/"+id, nil)
		if err == nil {
			var s sandbox
			if json.Unmarshal(data, &s) == nil {
				if contains(want, s.Status.State) {
					return nil
				}
				if contains(failedStates, s.Status.State) {
					return fmt.Errorf("sandbox %s -> %s: %s", id, s.Status.State, s.Status.Message)
				}
			}
		}
		time.Sleep(700 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for sandbox %s -> %v", id, want)
}

// Endpoint returns the host target for an app port (re-fetch after resume).
func (o *OpenSandbox) Endpoint(ctx context.Context, id string, port int) (Target, error) {
	data, code, err := o.do(ctx, http.MethodGet, fmt.Sprintf("/sandboxes/%s/endpoints/%d", id, port), nil)
	if err != nil {
		return Target{}, err
	}
	if code >= 300 {
		return Target{}, fmt.Errorf("endpoint (%d): %s", code, string(data))
	}
	var e struct {
		Endpoint string `json:"endpoint"` // "host:port/prefix"
	}
	if err := json.Unmarshal(data, &e); err != nil || e.Endpoint == "" {
		return Target{}, fmt.Errorf("endpoint: bad response: %s", string(data))
	}
	hostport, prefix := e.Endpoint, ""
	if i := strings.Index(e.Endpoint, "/"); i >= 0 {
		hostport, prefix = e.Endpoint[:i], e.Endpoint[i:]
	}
	host, ps, _ := strings.Cut(hostport, ":")
	p, _ := strconv.Atoi(ps)
	return Target{Host: host, Port: p, Prefix: prefix}, nil
}

func (o *OpenSandbox) Pause(ctx context.Context, id string) error {
	_, code, err := o.do(ctx, http.MethodPost, "/sandboxes/"+id+"/pause", nil)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("pause failed (%d)", code)
	}
	return o.WaitState(ctx, id, pausedStates, 60*time.Second)
}

func (o *OpenSandbox) Resume(ctx context.Context, id string, port int) (Target, error) {
	if _, code, err := o.do(ctx, http.MethodPost, "/sandboxes/"+id+"/resume", nil); err != nil {
		return Target{}, err
	} else if code >= 300 {
		return Target{}, fmt.Errorf("resume failed (%d)", code)
	}
	if err := o.WaitState(ctx, id, runningStates, 60*time.Second); err != nil {
		return Target{}, err
	}
	return o.Endpoint(ctx, id, port)
}

func (o *OpenSandbox) Delete(ctx context.Context, id string) error {
	_, _, err := o.do(ctx, http.MethodDelete, "/sandboxes/"+id, nil)
	return err
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
