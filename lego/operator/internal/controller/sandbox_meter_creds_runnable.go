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

package controller

import (
	"context"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// sandboxMeterCredsMinter is the registry package's provisioning of the storage
// meter's two Zot users and their Secret.
type sandboxMeterCredsMinter interface {
	EnsureSandboxMeterCreds(ctx context.Context) error
}

const (
	// sandboxMeterCredsRetry is how soon a failed ensure is tried again (the
	// registry Secrets may not exist yet at the operator's first start).
	sandboxMeterCredsRetry = 30 * time.Second
	// sandboxMeterCredsResync re-ensures a healthy credential, so a deleted
	// Secret or a hand-edited Zot ACL converges without an operator restart.
	sandboxMeterCredsResync = 10 * time.Minute
)

// SandboxMeterCredsRunnable keeps the sandbox storage meter's Zot users
// provisioned (w22/m321, docs/ADR023 § Sandbox snapshot storage). Unlike the
// per-namespace snapshot credentials these are platform-wide, so nothing
// triggers a reconcile for them: the runnable ensures once at start, retries
// while that fails, and re-ensures periodically. Mechanism only: bex-api reads
// the resulting Secret.
type SandboxMeterCredsRunnable struct {
	Registry sandboxMeterCredsMinter
	// Retry and Resync override the defaults; tests shorten them.
	Retry, Resync time.Duration
}

// Start implements manager.Runnable and runs until ctx ends.
func (r *SandboxMeterCredsRunnable) Start(ctx context.Context) error {
	retry, resync := r.Retry, r.Resync
	if retry <= 0 {
		retry = sandboxMeterCredsRetry
	}
	if resync <= 0 {
		resync = sandboxMeterCredsResync
	}
	log := logf.Log.WithName("sandbox-meter-creds")
	for {
		wait := resync
		if err := r.Registry.EnsureSandboxMeterCreds(ctx); err != nil {
			log.Error(err, "ensuring the sandbox meter registry credentials failed; will retry")
			wait = retry
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}
