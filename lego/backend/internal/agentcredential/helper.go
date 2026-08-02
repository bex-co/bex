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

// Package agentcredential implements git's credential-helper protocol for the
// sandbox image. It fetches one short-lived GitHub installation token on each
// get request and has no cache or filesystem write path.
package agentcredential

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

type Config struct {
	GatewayURL string
	Namespace  string
	SessionID  string
	Repository string
	Branch     string
	HTTP       *http.Client
}

// Run handles get/store/erase. store and erase are deliberate no-ops so Git can
// never hand the token back to a cache. Only get writes a credential, to stdout.
func Run(ctx context.Context, operation string, stdin io.Reader, stdout io.Writer, cfg Config) error {
	fields, err := readCredentialInput(stdin)
	if err != nil {
		return err
	}
	switch operation {
	case "store", "erase":
		return nil
	case "get":
	default:
		return fmt.Errorf("unsupported git credential operation %q", operation)
	}
	if fields["protocol"] != "https" || !strings.EqualFold(fields["host"], "github.com") {
		return errors.New("git-credential-bex only serves https://github.com")
	}
	repository := cfg.Repository
	if repository == "" {
		repository = fields["path"]
	}
	repository, err = agentsession.NormalizeRepository(repository)
	if err != nil {
		return err
	}
	if cfg.GatewayURL == "" || cfg.Namespace == "" || cfg.SessionID == "" || cfg.Branch == "" {
		return errors.New("git-credential-bex is missing its session broker configuration")
	}
	endpoint, err := url.Parse(cfg.GatewayURL)
	if err != nil || endpoint.Scheme != "http" || endpoint.Host == "" {
		return errors.New("git-credential-bex gateway URL must be an internal http URL")
	}
	body, err := json.Marshal(agentsession.MintRequest{SessionID: cfg.SessionID, Repository: repository, Branch: cfg.Branch})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(agentsession.NamespaceHeader, cfg.Namespace)
	client := cfg.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("git-credential-bex could not reach the credential broker")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("git-credential-bex broker refused the request (status %d)", resp.StatusCode)
	}
	var credential agentsession.MintResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&credential); err != nil || credential.Token == "" || credential.Username == "" {
		return errors.New("git-credential-bex received an invalid broker response")
	}
	// This is the only point the token leaves process memory, over the helper's
	// stdout pipe directly into git. It is never placed in an error or a file.
	_, err = fmt.Fprintf(stdout, "username=%s\npassword=%s\n\n", credential.Username, credential.Token)
	return err
}

func readCredentialInput(r io.Reader) (map[string]string, error) {
	fields := map[string]string{}
	scanner := bufio.NewScanner(io.LimitReader(r, 16<<10))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			return nil, errors.New("invalid git credential input")
		}
		fields[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return fields, nil
}
