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

// Command kv-internal-proxy fronts a single managed Key Value store and holds
// its plan's connection limit. Internal clients inherit the tenant default ACL
// unless BEX_KV_INTERNAL_AUTH=client (the store requires internal
// authentication), when they authenticate themselves; external TLS remains end
// to end. The first SIGTERM or SIGINT drains for BEX_KV_DRAIN_SECONDS
// (w6/m512) and a second closes every stream at once.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/kvinternalauth"
)

func main() {
	check := flag.Bool("check", false, "check authenticated upstream readiness without opening listeners")
	flag.Parse()
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	drain := make(chan struct{})
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		close(drain)
		<-signals
		stop()
	}()
	if err := run(ctx, os.Getenv, *check, drain); err != nil {
		log.Fatal(err)
	}
}

// run serves until ctx ends, which closes every stream at once, or until both
// listeners have drained once drain closes.
func run(ctx context.Context, getenv func(string) string, check bool, drain <-chan struct{}) error {
	upstream, password := getenv("BEX_KV_UPSTREAM_ADDR"), getenv("VALKEY_PASSWORD")
	if upstream == "" || password == "" {
		return errors.New("kv-internal-proxy: BEX_KV_UPSTREAM_ADDR and VALKEY_PASSWORD are required")
	}
	mode, err := internalMode(getenv("BEX_KV_INTERNAL_AUTH"))
	if err != nil {
		return err
	}
	drainTimeout, err := drainSeconds(getenv("BEX_KV_DRAIN_SECONDS"))
	if err != nil {
		return err
	}
	if check {
		// Authenticated in both modes (kvinternalauth.Check).
		return kvinternalauth.Check(ctx, kvinternalauth.Config{Upstream: upstream, Password: password})
	}
	// One budget for both listeners: the store's plan limit (w6/m456).
	limit := 0
	if raw := getenv("BEX_KV_MAX_CONNECTIONS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return errors.New("kv-internal-proxy: BEX_KV_MAX_CONNECTIONS must be a positive integer")
		}
		limit = n
	}
	budget := kvinternalauth.NewBudget(limit)
	addr := getenv("BEX_KV_INTERNAL_ADDR")
	if addr == "" {
		addr = ":6379"
	}
	internal, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer func() { _ = internal.Close() }()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error, 2)
	count := 1
	if tlsAddr := getenv("BEX_KV_TLS_ADDR"); tlsAddr != "" {
		tlsUpstream := getenv("BEX_KV_TLS_UPSTREAM_ADDR")
		if tlsUpstream == "" {
			return errors.New("kv-internal-proxy: BEX_KV_TLS_UPSTREAM_ADDR is required with BEX_KV_TLS_ADDR")
		}
		listener, err := net.Listen("tcp", tlsAddr)
		if err != nil {
			return err
		}
		defer func() { _ = listener.Close() }()
		count++
		go func() {
			results <- kvinternalauth.Serve(ctx, listener, kvinternalauth.Config{
				Upstream: tlsUpstream, Mode: kvinternalauth.TLSPassthrough, Budget: budget,
				TLSCertFile: getenv("BEX_KV_TLS_CERT_FILE"), TLSKeyFile: getenv("BEX_KV_TLS_KEY_FILE"),
				Drain: drain, DrainTimeout: drainTimeout,
			})
		}()
	}
	go func() {
		results <- kvinternalauth.Serve(ctx, internal, kvinternalauth.Config{
			Upstream: upstream, Password: password, Mode: mode, Budget: budget,
			Drain: drain, DrainTimeout: drainTimeout,
		})
	}()
	if drainTimeout > 0 {
		go func() {
			select {
			case <-drain:
				kvinternalauth.KeepReserve(ctx, kvinternalauth.Config{Upstream: upstream, Password: password}, budget)
			case <-ctx.Done():
			}
		}()
	}
	var result error
	for range count {
		// A listener that has drained leaves the other to finish its own drain;
		// one that failed stops both.
		if err := <-results; err != nil {
			if result == nil {
				result = err
			}
			cancel()
		}
	}
	return result
}

// drainSeconds reads BEX_KV_DRAIN_SECONDS: a whole number of seconds, zero
// included. Unset means zero, which closes every stream at once on SIGTERM.
func drainSeconds(raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, errors.New("kv-internal-proxy: BEX_KV_DRAIN_SECONDS must be a whole number of seconds")
	}
	return time.Duration(n) * time.Second, nil
}

// internalMode is the plain listener's mode: the proxy authenticates its
// clients (unset, w8/204) or they authenticate themselves ("client": the store
// requires internal authentication, w6/m463). Anything else fails closed.
func internalMode(value string) (kvinternalauth.Mode, error) {
	switch value {
	case "":
		return kvinternalauth.InjectAuth, nil
	case "client":
		return kvinternalauth.PlainPassthrough, nil
	}
	return 0, errors.New(`kv-internal-proxy: BEX_KV_INTERNAL_AUTH must be unset or "client"`)
}
