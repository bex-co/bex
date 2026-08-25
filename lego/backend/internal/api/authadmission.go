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

package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// authadmission.go bounds the work an UNAUTHENTICATED caller can impose on the
// shared identity services (w1/m67 F1, from the 2026-08-10 codex-security scan).
//
// THE PROBLEM: the gate is mounted as auth(rl(handler)), and rl — the only
// general limiter — keys on the RESOLVED identity, so it cannot protect the work
// of resolving one. There is no negative cache (an inactive token is
// deliberately never cached, so a revocation is never masked), and singleflight
// only coalesces *identical* tokens. Every unique invalid bearer or session
// credential therefore costs exactly one Hydra introspection or Kratos whoami: a
// cheap anonymous loop amplifies into the services every tenant's control plane
// depends on.
//
// WHAT THIS METERS, AND WHY NOT REQUESTS: a per-request IP budget in front of
// the gate would be wrong here. The dashboard's SSR calls all arrive from ONE
// pod IP carrying each user's forwarded session cookie, and Kratos sessions are
// not positively cached, so such a budget would throttle the entire dashboard
// the moment it got busy — trading a hypothetical outage for a real one.
//
// So this combines two independent budgets: invalid credentials spend their
// source-IP bucket, while EVERY credential (valid or invalid) spends a bucket
// keyed by a process-secret HMAC of the actual bearer/session value. The latter
// rejects one abusive valid session before another upstream call without
// storing credentials or coupling unrelated SSR users behind one proxy IP.
//
// A global in-flight semaphore bounds the other axis: even admitted traffic
// cannot have more than N upstream auth calls outstanding at once.

// errAuthOverloaded is returned by the gate when a source has spent its
// invalid-credential budget, or when the process already has its maximum number
// of upstream auth calls in flight. Mapped to 429 + Retry-After.
var errAuthOverloaded = errors.New("authentication upstream budget exhausted")

const (
	// maxCredentialBytes bounds a bearer/session credential before it is copied
	// into an upstream request body or header. Real Hydra access tokens and
	// Kratos session tokens are well under 1 KiB; anything above this is either a
	// bug or an attempt to make bex allocate and forward megabytes per request.
	maxCredentialBytes = 4096

	// authAdmissionIdle/Sweep mirror the other limiters' housekeeping.
	authAdmissionIdle  = 10 * time.Minute
	authAdmissionSweep = 5 * time.Minute
)

// AuthAdmission is the per-source failure budget, per-credential request and
// concurrency budget, plus global concurrency bound
// described above. A nil *AuthAdmission admits everything, which is exactly the
// pre-m67 behavior.
type AuthAdmission struct {
	// failures meters INVALID credentials per client IP. nil ⇒ unmetered.
	failures *core.KeyedRateLimiter[string]
	// credentials meters every expensive upstream call by an HMAC fingerprint
	// of the presented credential. The raw bearer/session value is never retained.
	credentials           *core.KeyedRateLimiter[string]
	fingerprintKey        [32]byte
	credentialMaxInflight int
	credentialMu          sync.Mutex
	credentialInflight    map[string]int
	// inflight bounds concurrent upstream auth calls process-wide. nil ⇒ unbounded.
	inflight chan struct{}
	// TrustedProxies resolves the real client IP behind the edge proxies, so
	// every anonymous Internet caller does not share Traefik's single bucket.
	// Set by the composition root alongside the other IP-keyed budgets.
	TrustedProxies core.TrustedProxies
}

// NewAuthAdmission builds the bound from configuration. failuresPerMin ≤ 0
// disables the per-source budget; maxInflight ≤ 0 disables the concurrency
// bound; with both disabled it returns nil (feature off).
func NewAuthAdmission(failuresPerMin float64, burst, maxInflight int) *AuthAdmission {
	a := &AuthAdmission{
		failures:           core.NewKeyedRateLimiter[string](failuresPerMin, burst, authAdmissionIdle, authAdmissionSweep),
		credentials:        core.NewKeyedRateLimiter[string](failuresPerMin, burst, authAdmissionIdle, authAdmissionSweep),
		credentialInflight: map[string]int{},
	}
	_, _ = rand.Read(a.fingerprintKey[:])
	if maxInflight > 0 {
		a.inflight = make(chan struct{}, maxInflight)
		a.credentialMaxInflight = maxInflight / 8
		if a.credentialMaxInflight < 1 {
			a.credentialMaxInflight = 1
		}
	}
	if a.failures == nil && a.credentials == nil && a.inflight == nil {
		return nil
	}
	return a
}

// admit decides whether an upstream auth call may proceed for r's source and
// credential. It spends the per-credential request budget and reserves both
// per-credential and global in-flight slots. The returned release func MUST be
// called when the upstream call completes.
//
// ORDER (codex-security 2026-08 F1): the per-source failure budget is read
// BEFORE the per-credential bucket is created or spent. Bucket() lazily
// allocates a map entry per fingerprint, so consulting it first would let an
// anonymous caller convert each correctly-shed request (one distinct random
// bearer) into a permanent allocation. A shed request must perform no
// allocation.
func (a *AuthAdmission) admit(r *http.Request) (release func(), err error) {
	if a == nil {
		return func() {}, nil
	}
	if a.failures != nil && a.failures.Bucket(a.TrustedProxies.ClientIP(r)).Tokens() < 1 {
		return nil, errAuthOverloaded
	}
	credential := a.credentialFingerprint(r)
	if a.credentials != nil && !a.credentials.Bucket(credential).Allow() {
		return nil, errAuthOverloaded
	}
	releaseCredential, ok := a.acquireCredential(credential)
	if !ok {
		return nil, errAuthOverloaded
	}
	if a.inflight == nil {
		return releaseCredential, nil
	}
	select {
	case a.inflight <- struct{}{}:
		return func() {
			<-a.inflight
			releaseCredential()
		}, nil
	default:
		releaseCredential()
		// Shed rather than queue: a queued request holds a connection and its
		// upstream deadline while the flood continues. Overload must be visible.
		return nil, errAuthOverloaded
	}
}

func (a *AuthAdmission) credentialFingerprint(r *http.Request) string {
	credential := presentedCredential(r)
	value := credential.kind + ":" + credential.value
	if credential.kind == "" {
		value = "source:" + a.TrustedProxies.ClientIP(r)
	}
	mac := hmac.New(sha256.New, a.fingerprintKey[:])
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *AuthAdmission) acquireCredential(key string) (func(), bool) {
	if a == nil || a.credentialMaxInflight <= 0 {
		return func() {}, true
	}
	a.credentialMu.Lock()
	defer a.credentialMu.Unlock()
	if a.credentialInflight[key] >= a.credentialMaxInflight {
		return nil, false
	}
	a.credentialInflight[key]++
	return func() {
		a.credentialMu.Lock()
		defer a.credentialMu.Unlock()
		a.credentialInflight[key]--
		if a.credentialInflight[key] == 0 {
			delete(a.credentialInflight, key)
		}
	}, true
}

// penalize charges r's source one token for a credential that turned out to be
// invalid. Called only after the upstream verdict, so a legitimate caller — no
// matter how many users share its IP — is never charged.
func (a *AuthAdmission) penalize(r *http.Request) {
	if a == nil || a.failures == nil {
		return
	}
	a.failures.Bucket(a.TrustedProxies.ClientIP(r)).Allow()
}

// oversizedCredential reports whether a credential is too large to be worth
// forwarding upstream. Checked before any allocation or network call.
func oversizedCredential(s string) bool { return len(s) > maxCredentialBytes }

// writeAuthOverloaded answers a shed authentication attempt. It reuses the
// surface-aware 429 body the per-caller limiter writes, so a client cannot
// distinguish which limiter shed it.
func writeAuthOverloaded(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "1")
	writeTooManyRequests(w, r)
}
