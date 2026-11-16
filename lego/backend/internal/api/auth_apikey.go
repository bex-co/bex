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
	"context"
	"crypto/sha256"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// auth_apikey.go lets an API key be the bearer, as a Render API key is
// (docs/ADR012-auth.md §8e, w7/m426). A key is still a Hydra client_credentials
// client and Hydra still holds the only copy of its secret, as a hash. The gate
// does the exchange a caller used to do: it trades the key for an access token
// at Hydra's token endpoint, remembers that token for the key, and hands it to
// introspect. Everything after that is the path every OAuth token takes, so
// identity, workspace binding, the last-used stamp, audit attribution and
// revocation are those of the key's own client.
//
// The key reaches three places and no others: the Authorization header, the
// form body of the exchange, and SHA-256 as a cache key. It is never logged,
// never a metric label and never part of an error, and nothing that outlives
// the request shares the header's memory.

const (
	// keyTokenMargin is how long before its expiry a key's access token stops
	// being used, so a request never carries a token that expires on the way to
	// introspection.
	keyTokenMargin = time.Minute

	// keyKnownTTL is how long a key that has exchanged successfully on this
	// replica stays known. A known key is not held back by its client id's
	// failure budget, so wrong guesses at that id cannot lock its owner out
	// (see keyFailuresPerMinute).
	keyKnownTTL = 24 * time.Hour

	// keyRefusedTTL is how long a key Hydra refused is refused again without
	// asking. Each ask makes Hydra hash a secret, and a wrong key stays wrong:
	// a key's secret never changes and a revoked key never returns.
	keyRefusedTTL = time.Minute

	// keyFailuresPerMinute bounds the refused exchanges for one client id on
	// one replica, whatever their source: the distinct secrets a caller can try
	// against it at Hydra. Past it, a key this replica does not know is shed
	// with 429 before Hydra is asked.
	keyFailuresPerMinute = 10

	// maxTokenResponseBytes bounds the token endpoint's answer. Hydra's is a
	// few hundred bytes.
	maxTokenResponseBytes = 64 << 10
)

// The reasons a bearer key was not accepted, as the `reason` label of
// bex_api_apikey_bearer_failures_total: a closed set, so neither the key nor
// its client id can become a series.
const (
	keyFailureMalformed = "malformed" // carries the key prefix but is not a key
	keyFailureRefused   = "refused"   // Hydra refused the exchange
	keyFailureCached    = "cached"    // refused again from the negative cache
	keyFailureInactive  = "inactive"  // exchanged, but the token is not an active key
	keyFailureLimited   = "limited"   // shed by a failure budget (429)
)

// keyBearer is the gate's memory of the API keys presented as bearers. Each
// cache is keyed by keyHash, so the three bound themselves separately: a
// flood of wrong keys cannot evict the tokens of valid ones.
type keyBearer struct {
	tokenURL string
	client   *http.Client
	now      func() time.Time

	// tokens holds the access token a key last exchanged for, until that
	// token's use ends (keyTokenUse).
	tokens *core.TTLCache[keyToken]
	// known holds the keys that exchanged successfully within keyKnownTTL.
	known *core.TTLCache[struct{}]
	// refused holds the keys Hydra refused, for keyRefusedTTL.
	refused *core.TTLCache[struct{}]
	// group collapses concurrent exchanges of one key into one.
	group singleflight.Group
	// failures is the per-client-id budget of refused exchanges.
	failures *core.KeyedRateLimiter[string]
}

type keyToken struct {
	token    string
	clientID string
}

// presentedKey is the key of one request, parsed.
type presentedKey struct {
	hash     string // keyHash of the whole key
	clientID string
	secret   string
}

func newKeyBearer(issuer string, client *http.Client, now func() time.Time) *keyBearer {
	k := &keyBearer{
		tokenURL: strings.TrimSuffix(issuer, "/") + "/oauth2/token",
		client:   client,
		now:      now,
		tokens:   core.NewTTLCache[keyToken](),
		known:    core.NewTTLCache[struct{}](),
		refused:  core.NewTTLCache[struct{}](),
		failures: core.NewKeyedRateLimiter[string](keyFailuresPerMinute, 0, authAdmissionIdle, authAdmissionSweep),
	}
	k.tokens.Now, k.known.Now, k.refused.Now = now, now, now
	return k
}

// keyHash is the only form in which a key is held: its SHA-256, as raw bytes.
func keyHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return string(sum[:])
}

// authenticateKey resolves an API key sent as the bearer to its client's
// identity and to the access token that proves it. The zero Identity with a
// nil error is a refusal (401); a shed error (errAuthOverloaded) is a spent
// budget (429); any other error is Hydra being unable to answer (503).
func (a *oryAuth) authenticateKey(r *http.Request, bearer string) (core.Identity, string, error) {
	k := a.keys
	clientID, secret, ok := apikeys.ParseBearerKey(bearer)
	if !ok {
		return core.Identity{}, "", a.refuseKey(r, keyFailureMalformed)
	}
	key := presentedKey{hash: keyHash(bearer), clientID: clientID, secret: secret}
	for retried := false; ; retried = true {
		token, fresh, err := a.keyAccessToken(r, key)
		if err != nil || token == "" {
			return core.Identity{}, "", err
		}
		id, err := a.introspect(r, token)
		if err != nil {
			return core.Identity{}, "", err
		}
		// The token must be this key's own client, acting as itself.
		if id.ClientID == key.clientID && !id.Human {
			return id, token, nil
		}
		k.tokens.Delete(key.hash)
		if !fresh && !retried {
			// The remembered token died (its key was revoked, or the token was
			// revoked at Hydra alone). Ask Hydra once whether the key still
			// stands, so a live key never answers 401 for a dead token. The key
			// stays known for that ask.
			continue
		}
		if fresh {
			// A token Hydra issued a moment ago is not an active key: the key
			// was revoked through bex and its client outlives the tombstone.
			// Stop asking for a while.
			k.known.Delete(key.hash)
			k.refused.Put(key.hash, struct{}{}, k.now().Add(keyRefusedTTL))
		}
		a.metrics.observeAPIKeyFailure(keyFailureInactive)
		return core.Identity{}, "", nil
	}
}

// refuseKey answers a key that is refused without asking Hydra. The refusal
// still spends the source's failure budget, and a source that has none left
// gets a shed error (429), so repeating a bad key ends the way repeating
// any other invalid credential does. A key that is accepted from memory never
// comes here: a busy source's valid keys are not shed for its neighbour's
// failures.
func (a *oryAuth) refuseKey(r *http.Request, reason string) error {
	if a.admission.exhausted(r) {
		return a.shedKey(r, errShedSource)
	}
	a.admission.penalize(r)
	a.metrics.observeAPIKeyFailure(reason)
	return nil
}

// shedKey counts one key answered 429, under the budget that refused it, and
// returns that budget's error.
func (a *oryAuth) shedKey(r *http.Request, shed error) error {
	a.metrics.observeAPIKeyFailure(keyFailureLimited)
	a.metrics.observeAuthShed(credentialBearer, shed, a.admission.wasAuthenticated(r))
	return shed
}

// keyAccessToken returns the access token for a key: the remembered one while
// its use lasts, otherwise the result of an exchange (fresh). An empty token
// with a nil error means the key is refused.
func (a *oryAuth) keyAccessToken(r *http.Request, key presentedKey) (token string, fresh bool, err error) {
	k := a.keys
	if entry, ok := k.tokens.Get(key.hash); ok {
		return entry.token, false, nil
	}
	if _, refused := k.refused.Get(key.hash); refused {
		return "", false, a.refuseKey(r, keyFailureCached)
	}
	// An exchange is the expensive upstream call here, as a cache miss is for
	// introspection, and is metered the same way (w1/m67 F1). It is admitted
	// before anything is allocated for the key, so a source with no budget
	// left is shed at no cost.
	release, err := a.admission.admit(r)
	if err != nil {
		return "", false, a.shedKey(r, err)
	}
	defer release()
	// The parsed id is a slice of the request's Authorization header, which
	// holds the secret. What is kept past the request gets its own memory.
	key.clientID = strings.Clone(key.clientID)
	if _, known := k.known.Get(key.hash); !known && k.failures.Bucket(key.clientID).Tokens() < 1 {
		return "", false, a.shedKey(r, errShedKeyID)
	}
	token, err = core.SingleFlight(r.Context(), &k.group, key.hash, func(ctx context.Context) (string, error) {
		return k.exchange(ctx, key)
	})
	if err != nil {
		return "", false, err
	}
	if token == "" {
		a.admission.penalize(r)
		a.metrics.observeAPIKeyFailure(keyFailureRefused)
	}
	return token, true, nil
}

// exchange trades the key for an access token at Hydra's token endpoint
// (client_secret_post, the method keys are registered with) and records the
// outcome once for every caller waiting on it. An empty token with a nil error
// is Hydra's own refusal.
func (k *keyBearer) exchange(ctx context.Context, key presentedKey) (string, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {key.clientID},
		"client_secret": {key.secret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := k.client.Do(req)
	if err != nil {
		return "", err
	}
	defer core.DrainClose(resp)
	var out struct {
		AccessToken string  `json:"access_token"`
		ExpiresIn   float64 `json:"expires_in"`
		Error       string  `json:"error"`
	}
	// A body that is not a small JSON object leaves out empty, which the switch
	// below reads as neither a token nor a refusal.
	_ = core.DecodeLimitedJSON(resp.Body, maxTokenResponseBytes, &out)
	switch {
	case resp.StatusCode == http.StatusOK && out.AccessToken != "":
		use := keyTokenUse(time.Duration(out.ExpiresIn * float64(time.Second)))
		k.tokens.Put(key.hash, keyToken{token: out.AccessToken, clientID: key.clientID}, k.now().Add(use))
		k.known.Put(key.hash, struct{}{}, k.now().Add(keyKnownTTL))
		return out.AccessToken, nil
	case (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized) && out.Error != "":
		// Only an OAuth error from the token endpoint is a verdict on the key.
		// Anything else (an edge 403, a 429, a 5xx) is the endpoint failing to
		// answer, and must not be remembered as a refusal.
		k.tokens.Delete(key.hash)
		k.known.Delete(key.hash)
		k.refused.Put(key.hash, struct{}{}, k.now().Add(keyRefusedTTL))
		k.failures.Bucket(key.clientID).Allow()
		return "", nil
	default:
		return "", core.Err("hydra token endpoint returned " + resp.Status)
	}
}

// keyTokenUse is how long a token with the given lifetime is used: until
// keyTokenMargin before it expires, or for half its life when it is shorter
// than twice the margin. Hydra always states a lifetime; a token without one
// is used for the margin, and introspection still vets it every few seconds.
func keyTokenUse(lifetime time.Duration) time.Duration {
	switch {
	case lifetime <= 0:
		return keyTokenMargin
	case lifetime <= 2*keyTokenMargin:
		return lifetime / 2
	default:
		return lifetime - keyTokenMargin
	}
}

// forget drops the tokens remembered for a machine client bex just revoked,
// under every key string that reached them.
func (k *keyBearer) forget(identity core.Identity) {
	if k == nil || !identity.IsAPIKey() || identity.ClientID == "" {
		return
	}
	k.tokens.DeleteIf(func(entry keyToken) bool { return entry.clientID == identity.ClientID })
}
