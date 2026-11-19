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
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Draining on SIGTERM (w6/m485). Every platform release replaces both activator
// pods. A request held through a cold start (hold.go), or proxied to the App it
// woke, must still get the App's answer, and a request routed here while the
// pod's endpoint is withdrawn must still be served. Until w6/m485 the process
// had no signal handler, so the Go runtime exited at once on SIGTERM.

const (
	// defaultDrainWindow is how long a terminating activator keeps accepting
	// new connections. Kubernetes withdraws a terminating pod's endpoint by
	// itself (readiness is a TCP probe, so there is nothing to flip), and
	// Traefik and the nodes' service load balancing take a moment to stop
	// routing here. BEX_ACTIVATOR_DRAIN_SECONDS overrides it.
	defaultDrainWindow = 10 * time.Second
	// drainBound is how long the activator then waits for the requests it is
	// still serving. A request admitted at the end of the window waits up to
	// holdBound for its wake, and the margin is for the woken App's answer. A
	// proxied exchange or a hijacked stream (a WebSocket through a hold) still
	// open at the bound is cut. terminationGracePeriodSeconds in
	// config/activator/deployment.yaml covers the window plus this bound.
	drainBound = holdBound + 30*time.Second
)

// drainWindowFromEnv reads BEX_ACTIVATOR_DRAIN_SECONDS: a whole number of
// seconds, zero included; unset means defaultDrainWindow.
func drainWindowFromEnv() (time.Duration, error) {
	raw := os.Getenv("BEX_ACTIVATOR_DRAIN_SECONDS")
	if raw == "" {
		return defaultDrainWindow, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("BEX_ACTIVATOR_DRAIN_SECONDS must be a whole number of seconds, got %q", raw)
	}
	return time.Duration(n) * time.Second, nil
}

// handlersInFlight counts running handlers. http.Server.Shutdown stops waiting
// for a connection once it is hijacked, and the reverse proxy hijacks every
// upgraded one, so the drain also waits for the handler that copies it.
type handlersInFlight struct{ wg sync.WaitGroup }

func (f *handlersInFlight) track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.wg.Add(1)
		defer f.wg.Done()
		next.ServeHTTP(w, r)
	})
}

// wait returns once no handler is running, or ctx's error when it ends first.
func (f *handlersInFlight) wait(ctx context.Context) error {
	idle := make(chan struct{})
	go func() {
		f.wg.Wait()
		close(idle)
	}()
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// listenAndDrain serves handler on addr until SIGTERM or SIGINT, then drains
// it (serveUntilDrained); a second signal exits at once.
func listenAndDrain(log logr.Logger, addr string, handler http.Handler, window time.Duration) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Info("listening", "addr", ln.Addr().String())
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
	stop := ctrl.SetupSignalHandler()
	context.AfterFunc(stop, func() { log.Info("draining", "window", window, "bound", drainBound) })
	if err := serveUntilDrained(stop, srv, ln, window, drainBound); err != nil {
		return err
	}
	log.Info("drained")
	return nil
}

// serveUntilDrained serves srv on ln until stop ends. It then keeps accepting
// for window, stops accepting, and returns as soon as every open request has
// finished, or at bound with the context's error. A clean drain returns nil.
func serveUntilDrained(stop context.Context, srv *http.Server, ln net.Listener, window, bound time.Duration) error {
	var handlers handlersInFlight
	srv.Handler = handlers.track(srv.Handler)
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	select {
	case err := <-served:
		// Only the drain below shuts the server down, so this is a real
		// accept failure.
		return err
	case <-stop.Done():
	}
	time.Sleep(window)
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	defer cancel()
	err := srv.Shutdown(ctx)
	if err == nil {
		err = handlers.wait(ctx)
	}
	if err != nil {
		return fmt.Errorf("drain cut requests still open at the %v bound: %w", bound, err)
	}
	return nil
}
