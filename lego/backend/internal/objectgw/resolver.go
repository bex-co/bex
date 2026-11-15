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

package objectgw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/publichost"
)

var (
	// ErrNotFound is a host that is not a public bucket's: private, deleted,
	// unknown or malformed. The gateway answers all of them 404.
	ErrNotFound = errors.New("not a public bucket")
	// ErrBusy is a lookup the gateway will not make because it is over its rate
	// of lookups for hosts it has not seen: the cost of a guess at a host is one
	// call to bex-api, and an attacker has unlimited guesses.
	ErrBusy = errors.New("too many lookups")
)

const (
	// LookupTTL is how long an answer is kept, a miss included (ADR112 D4). It is
	// also how long disabling dev-url takes to reach every visitor.
	LookupTTL = 30 * time.Second
	// MaxLookupEntries bounds the answers held.
	MaxLookupEntries = 8192
	// LookupsPerSecond and LookupBurst bound the calls to bex-api for hosts not
	// known to be buckets; RenewalsPerSecond and RenewalBurst bound the refresh
	// of buckets that were. Both are bex's own and unverified (ADR018).
	LookupsPerSecond  = 50
	LookupBurst       = 100
	RenewalsPerSecond = 200
	RenewalBurst      = 400
	// maxLookupAnswerBytes bounds the answer read: 100 CORS rules in at most 64 KiB.
	maxLookupAnswerBytes = 256 << 10
)

// Bucket is a public bucket as the gateway needs it.
type Bucket struct {
	ID          string
	WorkspaceID string
	Region      objectstorage.Region
	CORS        []objectstorage.CORSRule
}

// Resolver turns a request host into the public bucket it names, or
// ErrNotFound.
type Resolver interface {
	Lookup(ctx context.Context, host string) (Bucket, error)
}

// isBucketHost reports whether the first label of host has the shape of a
// bucket's, so a host that cannot be one costs no lookup.
func isBucketHost(host string) bool {
	label, _, found := strings.Cut(publichost.NormalizeHost(host), ".")
	if !found {
		return false
	}
	_, ok := publichost.BucketID(label)
	return ok
}

// HTTPLookup asks bex-api's internal listener which bucket a host names (verbs.go
// is its other side).
type HTTPLookup struct{ APIClient }

func (l *HTTPLookup) Lookup(ctx context.Context, host string) (Bucket, error) {
	resp, err := l.do(ctx, http.MethodGet, LookupPath+"?host="+url.QueryEscape(host), nil)
	if err != nil {
		return Bucket{}, err
	}
	defer drainClose(resp)
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Bucket{}, ErrNotFound
	default:
		return Bucket{}, fmt.Errorf("bex-api answered the lookup with status %d", resp.StatusCode)
	}
	var answer LookupAnswer
	if err := core.DecodeLimitedJSON(resp.Body, maxLookupAnswerBytes, &answer); err != nil {
		return Bucket{}, errors.New("bex-api answered the lookup with a body that is not JSON within its limit")
	}
	return bucketFromAnswer(answer)
}

// bucketFromAnswer checks what bex-api said before the gateway builds a Wasabi
// key from it: both ids must be of their kinds and the region one bex serves.
func bucketFromAnswer(a LookupAnswer) (Bucket, error) {
	if k, ok := ids.KindOf(a.BucketID); !ok || k != ids.ObjectBucket {
		return Bucket{}, errors.New("bex-api answered the lookup with a bucket id that is not one")
	}
	if k, ok := ids.KindOf(a.WorkspaceID); !ok || k != ids.Workspace {
		return Bucket{}, errors.New("bex-api answered the lookup with a workspace id that is not one")
	}
	region, err := objectstorage.ParseRegion(a.Region)
	if err != nil {
		return Bucket{}, errors.New("bex-api answered the lookup with a region the gateway does not serve")
	}
	return Bucket{ID: a.BucketID, WorkspaceID: a.WorkspaceID, Region: region, CORS: a.CORS}, nil
}

// CachedResolver keeps a Resolver's answers for LookupTTL, misses included, and
// bounds what a flood of guessed hosts can cost: concurrent lookups of one host
// share a call, and lookups for hosts it has not seen are rate-limited. A host it
// has served before is refreshed under a limiter of its own, so a flood of
// guesses cannot make real buckets answer 503 as their entries expire.
type CachedResolver struct {
	Upstream   Resolver
	TTL        time.Duration
	MaxEntries int

	now      func() time.Time
	guesses  *rate.Limiter // lookups of hosts not known to be buckets
	renewals *rate.Limiter // refreshes of a bucket that was served
	flight   singleflight.Group

	mu      sync.Mutex
	entries map[string]lookupEntry
}

type lookupEntry struct {
	bucket  Bucket
	found   bool
	expires time.Time
}

// NewCachedResolver wraps upstream with the production TTL, bound and rates.
func NewCachedResolver(upstream Resolver) *CachedResolver {
	return &CachedResolver{
		Upstream: upstream, TTL: LookupTTL, MaxEntries: MaxLookupEntries, now: time.Now,
		guesses:  rate.NewLimiter(LookupsPerSecond, LookupBurst),
		renewals: rate.NewLimiter(RenewalsPerSecond, RenewalBurst),
		entries:  map[string]lookupEntry{},
	}
}

func (c *CachedResolver) Lookup(ctx context.Context, host string) (Bucket, error) {
	key := publichost.NormalizeHost(host)
	e, held := c.get(key)
	if held && c.now().Before(e.expires) {
		if !e.found {
			return Bucket{}, ErrNotFound
		}
		return e.bucket, nil
	}
	limiter := c.guesses
	if held && e.found {
		limiter = c.renewals
	}
	v, err, _ := c.flight.Do(key, func() (any, error) {
		if !limiter.AllowN(c.now(), 1) {
			return nil, ErrBusy
		}
		// One caller's cancellation must not cancel the callers sharing the call.
		lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), bexAPITimeout)
		defer cancel()
		b, err := c.Upstream.Lookup(lookupCtx, key)
		switch {
		case err == nil:
			c.put(key, lookupEntry{bucket: b, found: true})
		case errors.Is(err, ErrNotFound):
			c.put(key, lookupEntry{})
		}
		return b, err
	})
	if err != nil {
		return Bucket{}, err
	}
	return v.(Bucket), nil
}

// get returns the entry for key, expired or not: an expired one still says which
// limiter its refresh belongs to.
func (c *CachedResolver) get(key string) (lookupEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	return e, ok
}

func (c *CachedResolver) put(key string, e lookupEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	e.expires = now.Add(c.TTL)
	if _, held := c.entries[key]; !held && len(c.entries) >= c.MaxEntries {
		for k, old := range c.entries {
			if !now.Before(old.expires) {
				delete(c.entries, k)
			}
		}
		// Still full of live answers: start over. A flood of guessed hosts pays
		// for it in lookups (rate-limited above), never in memory.
		if len(c.entries) >= c.MaxEntries {
			c.entries = map[string]lookupEntry{}
		}
	}
	c.entries[key] = e
}
