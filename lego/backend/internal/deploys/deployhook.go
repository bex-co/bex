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

package deploys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	// DeployHookTokenAnnotation persists the rotatable credential on the App CR.
	// An annotation keeps the feature available in both store-backed and CR-only
	// modes without changing the App CRD or involving the operator. Tenants never
	// receive Kubernetes credentials; bex-api is the only public reveal surface.
	DeployHookTokenAnnotation = "bex.co/deploy-hook-token"
	deployHookTokenPrefix     = "dhk-"
	deployHookTokenBytes      = 32 // 256 bits; intentionally stronger than xid

	// The v1 trigger budget is fixed and per token. A two-request burst tolerates
	// one immediate CI retry; sustained calls refill at six per minute. This is
	// independent of BEX_RATE_LIMIT because hook requests have no caller identity.
	DefaultDeployHookRPM   = 6.0
	DefaultDeployHookBurst = 2

	deployHookLimiterIdle       = 10 * time.Minute
	deployHookLimiterSweepEvery = 5 * time.Minute
)

// DeployHookView is the identical REST/GraphQL/MCP management shape. URL is the
// credential: callers must handle it like an API key and avoid logging it.
type DeployHookView struct {
	URL string `json:"url"`
}

// newDeployHookToken mints a URL-safe, prefixed 256-bit credential. This is a
// secret rather than a resource id, so crypto/rand is the right primitive;
// internal/id's xid values are identifiers and provide only 96 bits.
func newDeployHookToken() (string, error) {
	b := make([]byte, deployHookTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("mint deploy-hook token: %w", err)
	}
	return deployHookTokenPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

func validDeployHookToken(token string) bool {
	raw, ok := strings.CutPrefix(token, deployHookTokenPrefix)
	if !ok {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	return err == nil && len(b) == deployHookTokenBytes
}

func deployHookTokenEqual(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *Service) deployHookURL(token string) string {
	path := "/v1/deploy-hooks/" + token
	if base := strings.TrimRight(s.DeployHookBaseURL, "/"); base != "" {
		return base + path
	}
	return path
}

// GetDeployHook returns (and lazily mints) a service's stable deploy-hook URL.
// Reading this credential requires the same sensitive-read relation as database
// connection strings. Lazy minting avoids touching every existing App at once.
func (s *Service) GetDeployHook(ctx context.Context, service string) (DeployHookView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanViewSensitive, service)
	if err != nil {
		return DeployHookView{}, err
	}
	token, err := s.writeDeployHookToken(ctx, a, false)
	if err != nil {
		return DeployHookView{}, err
	}
	return DeployHookView{URL: s.deployHookURL(token)}, nil
}

// RegenerateDeployHook atomically replaces the service's credential. Requests
// that resolve the old URL after this patch completes see no match and 404.
func (s *Service) RegenerateDeployHook(ctx context.Context, service string) (DeployHookView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, service)
	if err != nil {
		return DeployHookView{}, err
	}
	token, err := s.writeDeployHookToken(ctx, a, true)
	if err != nil {
		return DeployHookView{}, err
	}
	return DeployHookView{URL: s.deployHookURL(token)}, nil
}

// writeDeployHookToken returns the current token unless rotate is true. The
// optimistic-lock merge prevents concurrent first reads from returning two
// different URLs: a loser refetches and returns the winner's token.
func (s *Service) writeDeployHookToken(ctx context.Context, a *appv1alpha1.App, rotate bool) (string, error) {
	for range 5 {
		if !rotate {
			if token := a.Annotations[DeployHookTokenAnnotation]; validDeployHookToken(token) {
				return token, nil
			}
		}
		token, err := newDeployHookToken()
		if err != nil {
			return "", err
		}
		base := client.MergeFromWithOptions(a.DeepCopy(), client.MergeFromWithOptimisticLock{})
		if a.Annotations == nil {
			a.Annotations = map[string]string{}
		}
		a.Annotations[DeployHookTokenAnnotation] = token
		if err := s.Client.Patch(ctx, a, base); err == nil {
			return token, nil
		} else if !apierrors.IsConflict(err) {
			return "", err
		}
		if err := s.Client.Get(ctx, client.ObjectKeyFromObject(a), a); err != nil {
			return "", err
		}
		// A rotate that lost a conflict still must replace the value it refetched;
		// a lazy mint returns the concurrent winner on the next loop iteration.
	}
	return "", fmt.Errorf("rotate deploy-hook token: too many concurrent App updates")
}

// appForDeployHookToken resolves a credential without an identity. Unknown,
// malformed, and stale tokens intentionally collapse to the same 404.
func (s *Service) appForDeployHookToken(ctx context.Context, token string) (*appv1alpha1.App, error) {
	if !validDeployHookToken(token) {
		return nil, core.ErrNotFound
	}
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, client.InNamespace(s.Namespace)); err != nil {
		return nil, err
	}
	for i := range list.Items {
		want := list.Items[i].Annotations[DeployHookTokenAnnotation]
		if validDeployHookToken(want) && deployHookTokenEqual(token, want) {
			return list.Items[i].DeepCopy(), nil
		}
	}
	return nil, core.ErrNotFound
}

func deployHookServiceName(a *appv1alpha1.App) string {
	if name := a.Labels[core.LabelServiceName]; name != "" {
		return name
	}
	return a.Name
}

// DeployHookRateLimiter is a token-keyed in-memory bucket. Like the main API
// limiter it is replica-local; bex-api currently runs one replica. Keys are
// SHA-256 digests so the raw credential is not retained in the limiter map.
type DeployHookRateLimiter struct {
	mu        sync.Mutex
	entries   map[[sha256.Size]byte]*deployHookLimitEntry
	rps       rate.Limit
	burst     int
	lastSweep time.Time
}

type deployHookLimitEntry struct {
	lim  *rate.Limiter
	last time.Time
}

// NewDeployHookRateLimiter constructs a per-token limiter. Non-positive rpm
// disables it (nil), matching the main API limiter's constructor contract.
func NewDeployHookRateLimiter(rpm float64, burst int) *DeployHookRateLimiter {
	if rpm <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = int(math.Ceil(rpm))
	}
	return &DeployHookRateLimiter{
		entries: make(map[[sha256.Size]byte]*deployHookLimitEntry),
		rps:     rate.Limit(rpm / 60),
		burst:   burst,
	}
}

// reserve returns whether a request may proceed and, when denied, the delay
// until the next token is available.
func (rl *DeployHookRateLimiter) reserve(token string) (bool, time.Duration) {
	if rl == nil {
		return true, 0
	}
	key := sha256.Sum256([]byte(token))
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.lastSweep) > deployHookLimiterSweepEvery {
		for k, e := range rl.entries {
			if now.Sub(e.last) > deployHookLimiterIdle {
				delete(rl.entries, k)
			}
		}
		rl.lastSweep = now
	}
	e := rl.entries[key]
	if e == nil {
		e = &deployHookLimitEntry{lim: rate.NewLimiter(rl.rps, rl.burst)}
		rl.entries[key] = e
	}
	e.last = now
	res := e.lim.Reserve()
	if delay := res.Delay(); delay > 0 {
		res.Cancel()
		return false, delay
	}
	return true, 0
}

// DeployHookHandler serves the open credential-gated endpoint. It accepts GET
// and POST like Render, supports Render's `ref` commit query parameter, and
// returns Render's {deploy:{id}} success envelope. `imgURL` remains unsupported
// because bex has no safe image-origin matching/override verb yet.
func (s *Service) DeployHookHandler() http.Handler {
	limiter := s.DeployHookLimiter
	if limiter == nil {
		limiter = NewDeployHookRateLimiter(DefaultDeployHookRPM, DefaultDeployHookBurst)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET is deliberately mutating for Render/curl compatibility. Never let a
		// browser or intermediary cache the credential-bearing request or replay a
		// stale success without actually starting another deploy.
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, POST")
			core.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		token := r.PathValue("token")
		if token == "" { // makes direct handler tests useful outside a ServeMux
			token = strings.TrimPrefix(r.URL.Path, "/v1/deploy-hooks/")
		}
		a, err := s.appForDeployHookToken(r.Context(), token)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		if ok, retry := limiter.reserve(token); !ok {
			seconds := max(1, int(math.Ceil(retry.Seconds())))
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			core.WriteJSON(w, http.StatusTooManyRequests, map[string]any{
				"id":      "rate_limited",
				"message": "deploy hook rate limit exceeded; see Retry-After header",
			})
			return
		}
		if r.URL.Query().Get("imgURL") != "" {
			core.WriteErr(w, fmt.Errorf("%w: imgURL is not supported by bex deploy hooks", core.ErrBadRequest))
			return
		}
		d, err := s.triggerFetched(r.Context(), deployHookServiceName(a), a, TriggerParams{
			CommitID: r.URL.Query().Get("ref"),
		}, store.TriggerDeployHook)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]any{
			"deploy": map[string]string{"id": d.ID},
		})
	})
}
