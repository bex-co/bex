/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/render-oss/cli/pkg/client"
)

// agentSession is the part of bex-api's agent-session view `bex edit` reads
// (lego/backend/internal/agentsessions View). The CLI imports no sibling
// module, so testdata/agent-session-ssh-contract.json pins the field names.
type agentSession struct {
	ID         string `json:"id"`
	SSHAddress string `json:"sshAddress"`
}

// sessionNotRunning is the answer for a session with no live sandbox: the API
// omits sshAddress then.
const sessionNotRunning = "agent session %s has no running sandbox to open; send it a message to start one"

// sessionSSHAddress reads a session through the upstream client, exactly as a
// command would, including its token refresh.
func sessionSSHAddress(ctx context.Context, id string) (string, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", err
	}
	raw, ok := c.ClientInterface.(*client.Client)
	if !ok {
		return "", errors.New("unexpected API client")
	}
	return fetchSSHAddress(ctx, raw, id)
}

func fetchSSHAddress(ctx context.Context, c *client.Client, id string) (string, error) {
	endpoint, err := url.JoinPath(c.Server, "agent-sessions", id)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	for _, edit := range c.RequestEditors {
		if err := edit(ctx, req); err != nil {
			return "", err
		}
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("look up agent session %s: %w", id, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("look up agent session %s: %w", id, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("look up agent session %s: %s", id, apiMessage(resp.StatusCode, body))
	}
	var session agentSession
	if err := json.Unmarshal(body, &session); err != nil {
		return "", fmt.Errorf("look up agent session %s: %w", id, err)
	}
	if session.SSHAddress == "" {
		return "", fmt.Errorf(sessionNotRunning, id)
	}
	return session.SSHAddress, nil
}

// apiMessage prefers the API's own words: Render's `message`, or bex's
// `error` envelope.
func apiMessage(status int, body []byte) string {
	var envelope struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		for _, text := range []string{envelope.Message, envelope.Error} {
			if text = strings.TrimSpace(text); text != "" {
				return fmt.Sprintf("%d %s", status, text)
			}
		}
	}
	return fmt.Sprintf("%d %s", status, http.StatusText(status))
}
