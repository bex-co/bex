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
	"time"

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
	// DeployHookTokenDigestLabel is the non-secret lookup index for the token.
	// A URL credential is never a label value; its SHA-256 digest lets the API
	// server perform an exact selector instead of returning every tenant App.
	DeployHookTokenDigestLabel = "bex.co/deploy-hook-token-digest"
	deployHookTokenPrefix      = "dhk-"
	deployHookTokenBytes       = 32 // 256 bits; intentionally stronger than xid

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

func deployHookTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
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
		var token string
		if !rotate {
			token = a.Annotations[DeployHookTokenAnnotation]
			if validDeployHookToken(token) && a.Labels[DeployHookTokenDigestLabel] == deployHookTokenDigest(token) {
				return token, nil
			}
		}
		if !validDeployHookToken(token) || rotate {
			var err error
			token, err = newDeployHookToken()
			if err != nil {
				return "", err
			}
		}
		base := client.MergeFromWithOptions(a.DeepCopy(), client.MergeFromWithOptimisticLock{})
		if a.Annotations == nil {
			a.Annotations = map[string]string{}
		}
		a.Annotations[DeployHookTokenAnnotation] = token
		if a.Labels == nil {
			a.Labels = map[string]string{}
		}
		a.Labels[DeployHookTokenDigestLabel] = deployHookTokenDigest(token)
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

// BackfillDeployHookTokenDigests migrates pre-index Apps before the public hook
// route starts serving. It performs the one intentional cluster-wide list at
// startup, then patches only Apps that already carry a valid hook credential.
// Conflicts are retried and recompute the digest from the latest token, making
// concurrent replicas and rotations safe.
func BackfillDeployHookTokenDigests(ctx context.Context, cl client.Client) error {
	var list appv1alpha1.AppList
	if err := cl.List(ctx, &list); err != nil {
		return fmt.Errorf("list Apps for deploy-hook index backfill: %w", err)
	}
	for i := range list.Items {
		a := list.Items[i].DeepCopy()
		for range 5 {
			token := a.Annotations[DeployHookTokenAnnotation]
			if !validDeployHookToken(token) {
				break
			}
			digest := deployHookTokenDigest(token)
			if a.Labels[DeployHookTokenDigestLabel] == digest {
				break
			}
			base := client.MergeFromWithOptions(a.DeepCopy(), client.MergeFromWithOptimisticLock{})
			if a.Labels == nil {
				a.Labels = map[string]string{}
			}
			a.Labels[DeployHookTokenDigestLabel] = digest
			if err := cl.Patch(ctx, a, base); err == nil {
				break
			} else if !apierrors.IsConflict(err) {
				return fmt.Errorf("backfill deploy-hook index for %s/%s: %w", a.Namespace, a.Name, err)
			}
			if err := cl.Get(ctx, client.ObjectKeyFromObject(a), a); err != nil {
				return fmt.Errorf("refetch App during deploy-hook index backfill: %w", err)
			}
		}
		if token := a.Annotations[DeployHookTokenAnnotation]; validDeployHookToken(token) && a.Labels[DeployHookTokenDigestLabel] != deployHookTokenDigest(token) {
			return fmt.Errorf("backfill deploy-hook index for %s/%s: too many concurrent updates", a.Namespace, a.Name)
		}
	}
	return nil
}

// appForDeployHookToken resolves a credential without an identity. Unknown,
// malformed, and stale tokens intentionally collapse to the same 404.
func (s *Service) appForDeployHookToken(ctx context.Context, token string) (*appv1alpha1.App, error) {
	if !validDeployHookToken(token) {
		return nil, core.ErrNotFound
	}
	// Apps span tenant namespaces, so the selector is cluster-scoped but indexed:
	// the apiserver returns only the digest match rather than every tenant App.
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, client.MatchingLabels{
		DeployHookTokenDigestLabel: deployHookTokenDigest(token),
	}); err != nil {
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
// limiter (BEX_RATE_LIMIT) and the device-flow limiter, it is REPLICA-LOCAL by
// design: with bex-api's two replicas (w1/m52) the effective per-token ceiling
// is up to 2× DefaultDeployHookRPM. That is an accepted, bounded over-provision
// (w1/m58 audit): the endpoint is credential-gated (a leaked dhk- token, never
// an anonymous flood) and its action is newest-wins idempotent — extra triggers
// are superseded, not multiplied into extra builds — so a coarse per-replica
// ceiling is an abuse damper, not a security boundary. A shared control-plane
// counter was considered and rejected as disproportionate (a DB round trip per
// hook request to tighten a non-threat, and inconsistent with the other two
// replica-local limiters). Keys are SHA-256 digests so the raw credential is not
// retained in the limiter map.
type DeployHookRateLimiter struct {
	*core.KeyedRateLimiter[[sha256.Size]byte]
}

// NewDeployHookRateLimiter constructs a per-token limiter. Non-positive rpm
// disables it (nil), matching the main API limiter's constructor contract.
func NewDeployHookRateLimiter(rpm float64, burst int) *DeployHookRateLimiter {
	inner := core.NewKeyedRateLimiter[[sha256.Size]byte](rpm, burst, deployHookLimiterIdle, deployHookLimiterSweepEvery)
	if inner == nil {
		return nil
	}
	return &DeployHookRateLimiter{KeyedRateLimiter: inner}
}

// reserve returns whether a request may proceed and, when denied, the delay
// until the next token is available.
func (rl *DeployHookRateLimiter) reserve(token string) (bool, time.Duration) {
	if rl == nil {
		return true, 0
	}
	key := sha256.Sum256([]byte(token))
	lim := rl.Bucket(key)
	res := lim.Reserve()
	if delay := res.Delay(); delay > 0 {
		res.Cancel()
		return false, delay
	}
	return true, 0
}

// DeployHookHandler serves the open credential-gated endpoint. It accepts GET
// and POST like Render, supports Render's `ref` commit query parameter and
// `imgURL` image override, and returns Render's {deploy:{id}} success envelope.
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
			core.WriteErrStatus(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := r.PathValue("token")
		if token == "" { // makes direct handler tests useful outside a ServeMux
			token = strings.TrimPrefix(r.URL.Path, "/v1/deploy-hooks/")
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
		a, err := s.appForDeployHookToken(r.Context(), token)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		d, err := s.triggerFetched(r.Context(), deployHookServiceName(a), a, TriggerParams{
			CommitID: r.URL.Query().Get("ref"),
			ImageURL: r.URL.Query().Get("imgURL"),
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
