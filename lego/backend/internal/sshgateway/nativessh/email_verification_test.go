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

package nativessh

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// w2/m168: key auth never passes bex-api's middleware, so the gateway itself
// refuses a key whose owner's email is unverified — at authentication and again
// before every channel.
func TestUnverifiedKeyOwnerIsRefusedAtAuthentication(t *testing.T) {
	clientSigner := signer(t)
	var asked atomic.Value
	addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &countingExecutor{}, clientSigner, func(s *Server) {
		s.EmailVerified = func(_ context.Context, subject string) error {
			asked.Store(subject)
			return errors.New("unverified")
		}
	})
	defer stop()
	if client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner); err == nil {
		client.Close()
		t.Fatal("an unverified key owner authenticated")
	}
	if got, _ := asked.Load().(string); got != "user-1" {
		t.Fatalf("verification asked about %q, want the key's stored subject user-1", got)
	}
}

func TestVerificationLostAfterAuthenticationRefusesNextChannel(t *testing.T) {
	clientSigner := signer(t)
	var unverified atomic.Bool
	addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &countingExecutor{release: make(chan struct{})}, clientSigner, func(s *Server) {
		s.EmailVerified = func(context.Context, string) error {
			if unverified.Load() {
				return errors.New("unverified")
			}
			return nil
		}
	})
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatalf("verified owner refused: %v", err)
	}
	defer client.Close()
	unverified.Store(true)
	if _, err := client.NewSession(); err == nil {
		t.Fatal("a channel was accepted after the owner's verification was refused")
	}
}
