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

package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const (
	connectStateVersion = 1
	connectStateTTL     = 15 * time.Minute
	connectStateMaxLen  = 1024
)

var (
	errConnectStateMissing = errors.New("github connect state is missing")
	errConnectStateInvalid = errors.New("github connect state is invalid")
	errConnectStateExpired = errors.New("github connect state has expired")
)

// connectState is the short-lived credential that carries an already-authorized
// workspace through GitHub's cross-site install redirect. The whole payload plus
// its HMAC is base64url-encoded as one opaque query value; callers must never
// inspect or construct it themselves.
type connectState struct {
	Version     int    `json:"v"`
	WorkspaceID string `json:"workspaceId"`
	ExpiresAt   int64  `json:"expiresAt"`
}

// statefulInstallURL binds the install flow to the workspace StartConnect just
// authorized. The GitHub App private key's PEM bytes are reused as the HMAC key:
// they are already required, high-entropy, identical on every API replica, and
// remain on the platform-secret path described by ADR026.
func (s *Service) statefulInstallURL(workspaceID string) (string, error) {
	token, err := s.mintConnectState(workspaceID)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(s.installURL())
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("state", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *Service) mintConnectState(workspaceID string) (string, error) {
	if len(s.StateSecret) == 0 {
		return "", core.ErrGitHubUnavailable
	}
	if workspaceID == "" {
		return "", errConnectStateInvalid
	}
	payload, err := json.Marshal(connectState{
		Version:     connectStateVersion,
		WorkspaceID: workspaceID,
		ExpiresAt:   s.Now().Add(connectStateTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.StateSecret)
	_, _ = mac.Write(payload)
	signed := make([]byte, 0, len(payload)+sha256.Size)
	signed = append(signed, payload...)
	signed = append(signed, mac.Sum(nil)...)
	return base64.RawURLEncoding.EncodeToString(signed), nil
}

func (s *Service) verifyConnectState(token string) (string, error) {
	if len(s.StateSecret) == 0 {
		return "", core.ErrGitHubUnavailable
	}
	if token == "" {
		return "", errConnectStateMissing
	}
	if len(token) > connectStateMaxLen {
		return "", errConnectStateInvalid
	}
	signed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(signed) <= sha256.Size {
		return "", errConnectStateInvalid
	}
	payload, signature := signed[:len(signed)-sha256.Size], signed[len(signed)-sha256.Size:]
	mac := hmac.New(sha256.New, s.StateSecret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return "", errConnectStateInvalid
	}
	var state connectState
	if err := json.Unmarshal(payload, &state); err != nil || state.Version != connectStateVersion || state.WorkspaceID == "" || state.ExpiresAt <= 0 {
		return "", errConnectStateInvalid
	}
	if !s.Now().Before(time.Unix(state.ExpiresAt, 0)) {
		return "", errConnectStateExpired
	}
	return state.WorkspaceID, nil
}
