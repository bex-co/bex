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

// Command kv-internal-proxy fronts a single managed Key Value store. Internal
// clients inherit the tenant default ACL; external TLS remains end to end.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/bex-co/bex/lego/operator/internal/kvinternalauth"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	check := flag.Bool("check", false, "check authenticated upstream readiness without opening listeners")
	flag.Parse()
	if err := run(ctx, os.Getenv, *check); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, getenv func(string) string, check bool) error {
	upstream, password := getenv("BEX_KV_UPSTREAM_ADDR"), getenv("VALKEY_PASSWORD")
	if upstream == "" || password == "" {
		return errors.New("kv-internal-proxy: BEX_KV_UPSTREAM_ADDR and VALKEY_PASSWORD are required")
	}
	if check {
		return kvinternalauth.Check(ctx, kvinternalauth.Config{Upstream: upstream, Password: password})
	}
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
			results <- kvinternalauth.Serve(ctx, listener, kvinternalauth.Config{Upstream: tlsUpstream, Passthrough: true})
		}()
	}
	go func() {
		results <- kvinternalauth.Serve(ctx, internal, kvinternalauth.Config{Upstream: upstream, Password: password})
	}()
	var result error
	for range count {
		if err := <-results; err != nil && result == nil {
			result = err
		}
		cancel()
	}
	return result
}
