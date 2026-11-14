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

package main

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/api"
	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// noCreatorKeys is an API key store that cannot name a key's creator, standing
// in for the production store being swapped for, or wrapped in, a type that
// does not carry KeyCreator.
type noCreatorKeys struct{}

func (noCreatorKeys) Create(context.Context, string, string) (apikeys.APIKey, error) {
	return apikeys.APIKey{}, nil
}
func (noCreatorKeys) List(context.Context) ([]apikeys.APIKey, error)  { return nil, nil }
func (noCreatorKeys) Delete(context.Context, string) error            { return nil }
func (noCreatorKeys) Touch(context.Context, string, time.Time) error  { return nil }
func (noCreatorKeys) KeyOwner(context.Context, string) (string, bool) { return "", false }

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &buf
}

// Without KeyCreators every API-key call on an enforcing workspace would be a
// 503, so a store that lost it must be reported at startup, not discovered by
// the first customer.
func TestWireTwoFactorWarnsWhenTheAPIKeyStoreCannotNameACreator(t *testing.T) {
	logged := captureLog(t)
	deps := &api.Deps{APIKeys: noCreatorKeys{}}
	base := &core.Base{}

	wireTwoFactor(deps, base, nil, prometheus.NewRegistry())

	if !strings.Contains(logged.String(), "KeyCreator") {
		t.Fatalf("startup log = %q, want a warning that the API key store has no KeyCreator", logged.String())
	}
	if base.TwoFactor == nil || deps.TwoFactorChanged == nil {
		t.Fatal("the gate is still wired: only API-key principals lose their lookup")
	}
}

func TestWireTwoFactorIsQuietForTheProductionAPIKeyStore(t *testing.T) {
	logged := captureLog(t)
	wireTwoFactor(&api.Deps{APIKeys: apikeys.NewHydraAPIKeys("http://hydra.invalid")}, &core.Base{}, nil, prometheus.NewRegistry())
	if logged.Len() != 0 {
		t.Fatalf("startup log = %q, want nothing for the production store", logged.String())
	}
	// Nor is a deployment without API keys worth a warning.
	wireTwoFactor(&api.Deps{}, &core.Base{}, nil, prometheus.NewRegistry())
	if logged.Len() != 0 {
		t.Fatalf("startup log = %q, want nothing without an API key store", logged.String())
	}
}
