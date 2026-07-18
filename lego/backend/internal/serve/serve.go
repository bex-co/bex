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

// Package serve is the shared HTTP serve/shutdown lifecycle for the backend
// binaries (w1/m30 + w1/m52). It closes the two halves of the roll-window race:
//
//   - w1/m30: on SIGTERM, drain in-flight requests instead of serving through
//     the whole termination grace period ([UntilShutdown]'s Shutdown call).
//   - w1/m52: on SIGTERM, keep ACCEPTING new requests for a drain window while
//     the readiness probe reports draining — Traefik/kube-proxy keep routing to
//     a terminating pod for a few seconds after the kill, and those late-routed
//     requests must succeed, not hit a closed listener (the 502s in the
//     2026-07-18 incident). The image is a static binary with no shell, so the
//     usual `preStop: exec sleep` is unavailable; the drain lives in-process.
package serve

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"
)

// Readiness is the /readyz answer: 200 until [Readiness.StartDraining], 503
// after. It is deliberately separate from /healthz (liveness) — flipping the
// one probe both point at would make the kubelet restart a pod that is merely
// draining. One Readiness is shared by every server in the process; the first
// server to begin draining flips it for all of them (the pod is one endpoint).
type Readiness struct {
	draining atomic.Bool
}

// StartDraining makes the handler answer 503 from now on. Idempotent.
func (r *Readiness) StartDraining() { r.draining.Store(true) }

// Draining reports whether StartDraining has been called.
func (r *Readiness) Draining() bool { return r.draining.Load() }

// Handler serves GET /readyz: 200 "ok" while ready, 503 "draining" after
// StartDraining. Regular request handling is unaffected either way — the whole
// point of the drain window is that real requests keep succeeding while the
// probe says "stop routing new work here".
func (r *Readiness) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if r.draining.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("draining"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

// Options bounds UntilShutdown's termination sequence. The zero value is the
// pre-m52 behavior: no drain window, 10s in-flight shutdown timeout.
type Options struct {
	// Readiness, when non-nil, is flipped to draining the moment ctx is
	// cancelled — before the drain window starts.
	Readiness *Readiness
	// DrainWindow is how long the server keeps serving NEW requests after ctx
	// is cancelled, so endpoints/ingress catch up with the readiness flip
	// before the listener closes. It must fit inside the pod's
	// terminationGracePeriodSeconds together with ShutdownTimeout.
	DrainWindow time.Duration
	// ShutdownTimeout bounds the in-flight drain (http.Server.Shutdown) after
	// the drain window. Zero means 10s, the w1/m30 budget.
	ShutdownTimeout time.Duration
}

const defaultShutdownTimeout = 10 * time.Second

// UntilShutdown runs srv.ListenAndServe in a goroutine and blocks until either
// the server stops on its own (a bind/startup error, returned as-is) or ctx is
// cancelled — on cancel it flips readiness, keeps serving through the drain
// window, then gracefully shuts the server down within ShutdownTimeout and
// returns Shutdown's result (nil on a clean drain).
func UntilShutdown(ctx context.Context, srv *http.Server, opts Options) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	select {
	case err := <-serveErr:
		// The server stopped before any signal — e.g. the port was already
		// bound. ErrServerClosed only arises from the Shutdown in the ctx.Done
		// path below, so here it means a clean stop; anything else is a real
		// startup error the caller should fail fast on.
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		if opts.Readiness != nil {
			opts.Readiness.StartDraining()
		}
		if opts.DrainWindow > 0 {
			time.Sleep(opts.DrainWindow)
		}
		timeout := opts.ShutdownTimeout
		if timeout <= 0 {
			timeout = defaultShutdownTimeout
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
