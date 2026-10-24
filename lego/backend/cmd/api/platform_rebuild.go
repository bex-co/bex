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
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/deploys"
)

// platformRebuildCommand is the operator-run fleet rebuild that ships a
// re-pinned runtime to running services, run inside the bex-api pod:
//
//	kubectl -n bex-system exec deploy/bex-api -- /api platform-rebuild --since 2026-10-23T09:33:41Z [--apply] [--limit 10]
//
// It is a thin client of the running server's :8091 verb (the deploy path it
// needs — GitHub clone credentials, pull secrets — is wired there), so the
// control-plane token never leaves the pod. Without --apply it prints the plan
// and opens nothing.
const platformRebuildCommand = "platform-rebuild"

// lateHandler answers 503 until set: the :8091 mux starts serving before
// api.NewServer builds the deploy service the platform-rebuild verb runs on.
type lateHandler struct{ h atomic.Pointer[http.Handler] }

func (l *lateHandler) set(h http.Handler) { l.h.Store(&h) }

func (l *lateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h := l.h.Load(); h != nil {
		(*h).ServeHTTP(w, r)
		return
	}
	core.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "starting"})
}

func parsePlatformRebuildArgs(args []string) (deploys.PlatformRebuildRequest, error) {
	fs := flag.NewFlagSet(platformRebuildCommand, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	since := fs.String("since", "", "RFC 3339 time the patched toolchain finished rolling out")
	apply := fs.Bool("apply", false, "open the deploys (default: print the plan)")
	limit := fs.Int("limit", 10, "most deploys one applied batch opens")
	usage := fmt.Errorf("usage: %s --since <RFC3339> [--apply] [--limit N]", platformRebuildCommand)
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 || *limit < 1 {
		return deploys.PlatformRebuildRequest{}, usage
	}
	t, err := time.Parse(time.RFC3339, *since)
	if err != nil {
		return deploys.PlatformRebuildRequest{}, usage
	}
	return deploys.PlatformRebuildRequest{Since: t, Apply: *apply, Limit: *limit}, nil
}

// loopbackURL is the in-pod address of the :8091 listener bound at addr.
func loopbackURL(addr string) (string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("BEX_CP_ADDR %q: %w", addr, err)
	}
	return "http://" + net.JoinHostPort("127.0.0.1", port), nil
}

// runPlatformRebuild posts one batch to the local :8091 verb, prints the JSON
// result, and returns the exit code.
func runPlatformRebuild(ctx context.Context, cfg *Config, args []string, out, errOut io.Writer) int {
	req, err := parsePlatformRebuildArgs(args)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if cfg.CPToken == "" {
		fmt.Fprintln(errOut, "BEX_CP_TOKEN is required: the verb is served by the control plane")
		return 2
	}
	base, err := loopbackURL(cfg.CPAddr)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	body, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, base+deploys.PlatformRebuildPath, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	hr.Header.Set("Authorization", "Bearer "+cfg.CPToken)
	hr.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(hr)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(out, resp.Body); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(errOut, "%s: %s\n", platformRebuildCommand, resp.Status)
		return 1
	}
	return 0
}
