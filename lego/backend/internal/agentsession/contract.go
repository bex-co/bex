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

// Package agentsession owns the internal credential-broker contract shared by
// bex-api and the isolated SSH gateway's Pod-bound Git smart-HTTP proxy. It
// deliberately exposes no raw credential to the sandbox and no public
// REST/GraphQL/MCP surface.
package agentsession

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

const (
	// InternalMintPath lives only on bex-api's cluster-internal :8091 listener.
	InternalMintPath = "/v1/agent-session-credentials"
	// GitProxyPath lives only on the gateway's cluster-internal :8082 listener.
	GitProxyPath = "/git/"

	SignatureHeader = "X-Bex-Agent-Credential-Signature"
	TimestampHeader = "X-Bex-Agent-Credential-Timestamp"
	NamespaceHeader = "X-Bex-Sandbox-Namespace"

	// Pod metadata is the authorization record until the durable session table
	// (w3/m39) lands. OpenSandbox copies create metadata to labels, so the repo and
	// branch use fixed-size digests while their exact values arrive in the request.
	LabelWorkspace  = "bex.co/workspace"
	LabelRegime     = "app.bex.co/regime"
	LabelSession    = "bex.co/agent-session"
	LabelRepository = "bex.co/agent-repository"
	LabelBranch     = "bex.co/agent-branch"
	RegimeSandbox   = "sandbox"

	BranchPrefix = "bex-agent/"
)

// SessionObject is the OpenFGA object literal for one durable agent session
// (deploy/gitops/authz/model.fga's agent_session type). Shared by the bex-api
// sandbox-exec seam (round-13 #1) and the gateway's exec revalidator so both
// sides check the identical object; the agentsessions feature's own
// sessionObject helper is this same string.
func SessionObject(sessionID string) string { return "agent_session:" + sessionID }

// ProxyRepositoryURL binds a Git smart-HTTP remote to the exact immutable
// session identity the gateway verifies against the direct source Pod. It
// contains no credential.
func ProxyRepositoryURL(baseURL, namespace, sessionID, repository, branch string) (string, error) {
	repository, err := NormalizeRepository(repository)
	if err != nil {
		return "", err
	}
	if namespace == "" || sessionID == "" {
		return "", ErrInvalidRequest
	}
	if err := ValidateBranch(branch); err != nil {
		return "", err
	}
	encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	return strings.TrimRight(baseURL, "/") + GitProxyPath + strings.Join([]string{
		encode(namespace), encode(sessionID), encode(repository), encode(branch),
	}, "/"), nil
}

var (
	ErrInvalidRequest = errors.New("invalid agent credential request")
	ErrForbidden      = errors.New("agent credential request forbidden")
)

// MintRequest contains no credential. The gateway derives Workspace, PodName,
// and PodUID from Kubernetes after matching the TCP source IP; the helper only
// supplies the non-secret session/repository/branch values bound by pod labels.
// Nonce is a per-request single-use token the client mints (Client.Mint) and the
// server claims once, riding inside the signed body so it is bound by the HMAC —
// it closes the ±skew replay window on this hop (security-audit run-1).
type MintRequest struct {
	SessionID  string `json:"sessionId"`
	Workspace  string `json:"workspace"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	PodName    string `json:"podName"`
	PodUID     string `json:"podUid"`
	Nonce      string `json:"nonce,omitempty"`
}

// MintResponse is intentionally the minimal git credential response. It is
// always emitted with Cache-Control: no-store and must never be logged.
type MintResponse struct {
	Username  string `json:"username"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

// NormalizeRepository accepts the owner/repo path Git passes to a credential
// helper and returns a stable GitHub repository identity. GitHub repository
// identities are case-insensitive; lowercasing prevents equivalent spellings
// from producing different pod-binding digests.
func NormalizeRepository(raw string) (string, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "/"))
	raw = strings.TrimSuffix(raw, ".git")
	parts := strings.Split(raw, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("%w: repository must be owner/name", ErrInvalidRequest)
	}
	for _, part := range parts {
		if part == "." || part == ".." || strings.ContainsAny(part, "\\\x00\r\n\t") {
			return "", fmt.Errorf("%w: invalid repository", ErrInvalidRequest)
		}
	}
	return strings.ToLower(parts[0] + "/" + parts[1]), nil
}

// ValidateBranch enforces ADR047 D2's server-side mint gate. This is a policy
// check, not a claim that GitHub installation tokens encode a branch scope;
// tenants must still protect default branches.
func ValidateBranch(branch string) error {
	if !strings.HasPrefix(branch, BranchPrefix) || len(branch) == len(BranchPrefix) || len(branch) > 255 {
		return fmt.Errorf("%w: branch must match %s*", ErrForbidden, BranchPrefix)
	}
	if strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".") || strings.Contains(branch, "..") || strings.Contains(branch, "@{") {
		return fmt.Errorf("%w: invalid branch", ErrForbidden)
	}
	for _, r := range branch {
		if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune("~^:?*[\\", r) {
			return fmt.Errorf("%w: invalid branch", ErrForbidden)
		}
	}
	return nil
}

// BindingLabels is the integration contract for the future session-create path:
// stamp these values into OpenSandbox create metadata. No secret is present.
func BindingLabels(sessionID, repository, branch string) (map[string]string, error) {
	repository, err := NormalizeRepository(repository)
	if err != nil {
		return nil, err
	}
	if err := ValidateBranch(branch); err != nil {
		return nil, err
	}
	if strings.TrimSpace(sessionID) == "" || len(sessionID) > 63 {
		return nil, fmt.Errorf("%w: session id is required", ErrInvalidRequest)
	}
	return map[string]string{
		LabelSession:    sessionID,
		LabelRepository: bindingDigest(repository),
		LabelBranch:     bindingDigest(branch),
	}, nil
}

// AuthorizePod binds a credential request to the source Pod resolved by the
// gateway. Namespace/workspace and every session target must match; a sibling
// sandbox can know these public values but its immutable labels will differ.
func AuthorizePod(namespace, podName, podUID string, labels map[string]string, req MintRequest) (MintRequest, error) {
	repository, err := NormalizeRepository(req.Repository)
	if err != nil {
		return MintRequest{}, err
	}
	if err := ValidateBranch(req.Branch); err != nil {
		return MintRequest{}, err
	}
	workspace := labels[LabelWorkspace]
	if workspace == "" || namespace != workspace+"-sandbox" || labels[LabelRegime] != RegimeSandbox ||
		labels[LabelSession] == "" || labels[LabelSession] != req.SessionID ||
		labels[LabelRepository] != bindingDigest(repository) || labels[LabelBranch] != bindingDigest(req.Branch) {
		return MintRequest{}, ErrForbidden
	}
	req.Workspace = workspace
	req.Repository = repository
	req.PodName = podName
	req.PodUID = podUID
	return req, nil
}

func bindingDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	// 52 lowercase base32 characters: full SHA-256 strength within Kubernetes'
	// 63-character label-value ceiling.
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
}
