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

package sandboxfiles

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
)

func TestFileTicketDomainAndPathBinding(t *testing.T) {
	now := time.Now()
	secret := []byte("test-only-shared-secret")
	c := Claims{Subject: "caller", Workspace: "tea-a", SandboxID: "os-a", Namespace: "tea-a-sandbox",
		Operation: OperationUpload, Path: "/workspace/a b.txt", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()}
	token, err := Mint(secret, c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Verify(secret, token, now)
	if err != nil || got.Path != c.Path || got.Operation != c.Operation || got.Nonce == "" {
		t.Fatalf("round trip = %#v, %v", got, err)
	}
	if _, err := Verify(secret, token, now.Add(2*time.Minute)); !errors.Is(err, ErrExpired) {
		t.Fatalf("expiry = %v", err)
	}
	if _, err := sandboxexec.Verify(secret, token, now); err == nil {
		t.Fatal("file ticket authorized arbitrary exec")
	}
	execToken, err := sandboxexec.Mint(secret, sandboxexec.Claims{Subject: "caller", SandboxID: "os-a", Namespace: "tea-a-sandbox", Command: []string{"sh"}, ExpiresAt: now.Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(secret, execToken, now); err == nil {
		t.Fatal("exec ticket authorized file transfer")
	}
	for _, mutate := range []func(*Claims){
		func(c *Claims) { c.Operation = "stream" },
		func(c *Claims) { c.Path = "/workspace/../secret" },
		func(c *Claims) { c.Namespace = "tea-other-sandbox" },
		func(c *Claims) { c.SandboxID = "../pod" },
	} {
		bad := c
		mutate(&bad)
		token, err := Mint(secret, bad)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Verify(secret, token, now); !errors.Is(err, ErrMalformed) {
			t.Fatalf("bad claims accepted: %#v, %v", bad, err)
		}
	}
}

func TestValidatePath(t *testing.T) {
	for _, value := range []string{"", "/", "relative", "/a/../b", "/a/./b", "/a//b", "/a/", "/a\x00b", "/a\nb", "/a\\b", "/" + strings.Repeat("x", MaxPathBytes), "/" + strings.Repeat("a/", MaxPathDepth) + "b"} {
		if ValidatePath(value) == nil {
			t.Errorf("accepted unsafe path %q", value)
		}
	}
	for _, value := range []string{"/workspace/file", "/tmp/空 白", "/tmp/$(literal);'name"} {
		if err := ValidatePath(value); err != nil {
			t.Errorf("rejected safe literal path %q: %v", value, err)
		}
	}
}
