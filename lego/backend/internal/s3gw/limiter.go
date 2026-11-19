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

package s3gw

import (
	"errors"
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/types/netutil"
)

// An unauthenticated request must cost little (ADR112 D3): the key id's shape is
// checked before any lookup, an unknown id is one database lookup whose miss is
// cached for 30 seconds in a bounded cache, a known key's secret is read from
// OpenBao at most once per 15 minutes on each replica, and failed authentications
// are limited per client address. Once an address has used its budget, its
// requests are authenticated from the in-memory caches alone, so a flood of
// unknown key ids from it reaches neither Postgres nor OpenBao.

// authFailures counts failed authentications per client address.
type authFailures struct {
	rl *core.KeyedRateLimiter[string]
}

func newAuthFailures() *authFailures {
	return &authFailures{rl: core.NewKeyedRateLimiter[string](objectstorage.MaxFailedAuthsPerMinute, objectstorage.MaxFailedAuthsPerMinute, 10*time.Minute, time.Minute)}
}

// exhausted reports whether the address has no failures left, without spending one.
func (a *authFailures) exhausted(key string) bool { return a.rl.Bucket(key).Tokens() < 1 }

// record counts one failed authentication.
func (a *authFailures) record(key string) { a.rl.Bucket(key).Allow() }

// clientKey is the key of the address a request came from.
func (g *Gateway) clientKey(r *http.Request) string {
	return netutil.SourceKeyString(g.cfg.TrustedProxies.ClientIP(r))
}

// isAuthFailure reports whether e is a failed authentication, the only answers
// that spend an address's budget: a wrong secret and an unknown key are the
// guesses a flood makes, while a skewed clock, an expired URL or an unreachable
// OpenBao are not the address's doing.
func isAuthFailure(e *Error) bool {
	return e != nil && (e.Code == "InvalidAccessKeyId" || e.Code == "SignatureDoesNotMatch" ||
		e.Code == "AuthorizationHeaderMalformed" || e.Code == "AuthorizationQueryParametersError")
}

// errTooManyFailures is the answer to an address over its budget whose key the
// caches do not hold. SlowDown is the throttle S3 clients retry.
func errTooManyFailures() *Error {
	return errSlowDown("Too many failed authentications from this address; retry later.")
}

// lookupFromCaches authenticates from memory alone and maps what it finds to the
// answers Authenticate gives, so an address over its budget costs neither store.
func (g *Gateway) lookupFromCaches(keyID string) (objectstorage.Credential, *Error) {
	cred, err := g.cfg.Keys.AuthenticateCached(keyID)
	switch {
	case err == nil:
		return cred, nil
	case errors.Is(err, objectstorage.ErrInvalidAccessKey):
		return objectstorage.Credential{}, errInvalidAccessKeyID()
	}
	return objectstorage.Credential{}, errTooManyFailures()
}

// requestLimits are the S3 surface's request rates (ADR112 D3, D8), per replica: 100 a
// second per workspace (burst 200) and 50 a second per access key (burst 100). They
// bound a client that opens thousands of tiny requests; the stream caps and the pacing
// bound the bytes. A refusal is 503 SlowDown, which SDKs retry, never 429.
type requestLimits struct {
	perWorkspace *core.KeyedRateLimiter[string]
	perKey       *core.KeyedRateLimiter[string]
	listings     *core.KeyedRateLimiter[string]
}

func newRequestLimits() *requestLimits {
	return &requestLimits{
		perWorkspace: core.NewKeyedRateLimiter[string](objectstorage.S3RequestsPerSecondPerWorkspace*60, objectstorage.S3RequestBurstPerWorkspace, 10*time.Minute, time.Minute),
		perKey:       core.NewKeyedRateLimiter[string](objectstorage.S3RequestsPerSecondPerKey*60, objectstorage.S3RequestBurstPerKey, 10*time.Minute, time.Minute),
		listings:     core.NewKeyedRateLimiter[string](objectstorage.ListingsPerSecondPerWorkspace*60, objectstorage.ListingsPerSecondPerWorkspace, 10*time.Minute, time.Minute),
	}
}

// allow spends one request of the key's and the workspace's budgets.
func (l *requestLimits) allow(keyID, workspaceID string) bool {
	return l.perKey.Bucket(keyID).Allow() && l.perWorkspace.Bucket(workspaceID).Allow()
}

// allowListing spends one listing of the workspace's budget: Wasabi counts what a
// listing reads as egress, so listings are capped on their own.
func (l *requestLimits) allowListing(workspaceID string) bool {
	return l.listings.Bucket(workspaceID).Allow()
}
