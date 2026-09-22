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

// Package sandboxfiles defines the path-bound file-transfer ticket and limits
// shared by bex-api and the isolated gateway. File tickets cannot authorize exec.
package sandboxfiles

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
)

const (
	TicketHeader                 = "X-Bex-Sandbox-File-Ticket"
	GatewayPath                  = "/sandbox-files"
	OperationUpload              = "upload"
	OperationDownload            = "download"
	MaxTransferBytes       int64 = 256 << 20
	MaxArchiveEntries            = 10_000
	MaxPathBytes                 = 4096
	MaxPathDepth                 = 64
	DefaultTransferTimeout       = 5 * time.Minute
)

type Claims struct {
	Subject        string `json:"sub"`
	Workspace      string `json:"ws"`
	SandboxID      string `json:"sbx"`
	Namespace      string `json:"ns"`
	AgentSessionID string `json:"ags,omitempty"`
	Operation      string `json:"op"`
	Path           string `json:"path"`
	IssuedAt       int64  `json:"iat"`
	ExpiresAt      int64  `json:"exp"`
	Nonce          string `json:"jti"`
}

var codec = hmacticket.New("sandbox file ticket")

var (
	ErrMalformed = codec.Malformed()
	ErrSignature = codec.Signature()
	ErrExpired   = codec.Expired()
)

func key(secret []byte) []byte {
	if len(secret) == 0 {
		return nil
	}
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte("bex sandbox file gateway ticket v1"))
	return h.Sum(nil)
}

func Mint(secret []byte, claims Claims) (string, error) {
	if err := hmacticket.EnsureNonce(&claims.Nonce); err != nil {
		return "", err
	}
	return codec.Sign(key(secret), claims)
}

func Verify(secret []byte, token string, now time.Time) (Claims, error) {
	var c Claims
	if err := codec.Open(key(secret), token, &c); err != nil {
		return Claims{}, err
	}
	if c.Subject == "" || c.Workspace == "" || c.SandboxID == "" || c.Namespace != c.Workspace+"-sandbox" ||
		strings.ContainsAny(c.SandboxID, "/\\") || c.Nonce == "" || c.ExpiresAt == 0 ||
		(c.Operation != OperationUpload && c.Operation != OperationDownload) || ValidatePath(c.Path) != nil {
		return Claims{}, ErrMalformed
	}
	if err := codec.CheckBounds(now, c.IssuedAt, c.ExpiresAt); err != nil {
		return Claims{}, err
	}
	return c, nil
}

func (c Claims) NonceExpiry() time.Time { return hmacticket.NonceExpiry(c.ExpiresAt) }
func (c Claims) PodName() string        { return c.SandboxID + "-0" }

// ValidatePath requires one exact absolute sandbox path. Rejecting instead of
// silently cleaning preserves the byte binding of the public connect token.
func ValidatePath(value string) error {
	if value == "/" || !path.IsAbs(value) || len(value) > MaxPathBytes || path.Clean(value) != value ||
		strings.ContainsRune(value, '\\') || strings.IndexFunc(value, unicode.IsControl) >= 0 ||
		strings.Count(value, "/") > MaxPathDepth {
		return fmt.Errorf("sandbox path must be a clean absolute non-root path, at most %d bytes and %d levels, without control characters", MaxPathBytes, MaxPathDepth)
	}
	return nil
}
