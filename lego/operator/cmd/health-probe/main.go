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

// Command health-probe is the per-pod HTTP health check the operator projects
// as an exec probe on a service with a Health Check Path (w8/m133;
// internal/healthprobe). "install DEST" copies this binary into the pod's
// shared volume (the init container); "check" runs one no-redirect request
// and exits 0 for a 2xx/3xx answer; "open-port" exits 0 when the pod accepts a
// TCP connection on any port it listens on (w4/m350), Render's default check;
// "tcp" exits 0 when the pod accepts a connection on one port, the kubelet's
// TCP probe. "drain --seconds N CHECK…" is the preStop hook (w4/m353): it runs
// one of those checks and waits N seconds only when it passes, so an instance
// still serving drains while one the health check failed restarts at once.
// A failed check prints one line in Render's words (w6/m634), which the
// kubelet puts in the pod's Unhealthy event.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/healthprobe"
)

// podAddrs, drainSleep and failures are seams for the tests. failures is
// where a failed check prints its line; the kubelet keeps stdout and stderr
// alike.
var (
	podAddrs             = healthprobe.PodAddrs
	drainSleep           = time.Sleep
	failures   io.Writer = os.Stderr
)

// outputMargin is how much sooner than its --timeout a check gives up. The
// kubelet's probe timeout is the same 5 s, counted from the exec's start, and
// the kubelet kills a command that overruns it and keeps none of its output,
// so a check that waited the whole 5 s would never print its own failure.
const outputMargin = 250 * time.Millisecond

// checkContext bounds a check whose --timeout is timeout, outputMargin short
// of it when that leaves the check most of its time.
func checkContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 4*outputMargin {
		timeout -= outputMargin
	}
	return context.WithTimeout(context.Background(), timeout)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: health-probe install DEST | check --port N --path P [--host H] [--timeout D]"+
			" | open-port [--skip P,Q] [--timeout D] | tcp --port N [--timeout D] | drain --seconds N CHECK [ARGS…]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "install":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: health-probe install DEST")
			os.Exit(2)
		}
		if err := healthprobe.Install(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "health-probe install:", err)
			os.Exit(1)
		}
	case "drain":
		drain(os.Args[2:])
	default:
		os.Exit(runCheck(os.Args[1:]))
	}
}

// runCheck runs one check subcommand: args[0] names it, the rest are its flags.
func runCheck(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "health-probe: no check named")
		return 2
	}
	switch args[0] {
	case "check":
		return check(args[1:])
	case "open-port":
		return openPort(args[1:])
	case "tcp":
		return tcp(args[1:])
	}
	fmt.Fprintf(os.Stderr, "health-probe: unknown command %q\n", args[0])
	return 2
}

// drain is the preStop hook of an instance that may still serve (w4/m353).
// Kubernetes runs preStop before every container stop: a deletion (a deploy,
// a restart, a scale-in), where the instance keeps serving while its endpoint
// leaves the routing, and a liveness kill, where readiness has already taken
// it out of rotation. It runs the instance's own health check once: a pass
// waits the drain, a failure returns at once so the restart comes at Render's
// 60 s. It has no failure exit, bad arguments included: the hook must never
// be what holds up termination.
func drain(args []string) {
	fs := flag.NewFlagSet("drain", flag.ContinueOnError)
	seconds := fs.Int("seconds", 0, "drain length in seconds")
	if err := fs.Parse(args); err != nil || *seconds <= 0 || fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "health-probe drain: bad arguments; not draining")
		return
	}
	if runCheck(fs.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "health-probe drain: the instance fails its health check, so it serves nothing to drain")
		return
	}
	drainSleep(time.Duration(*seconds) * time.Second)
}

// podAddr is the pod address the kubelet's probes dial: an app listening only
// on localhost is as unreachable here as it is from the edge.
func podAddr() (string, bool) {
	addrs := podAddrs()
	if len(addrs) == 0 {
		fmt.Fprintln(os.Stderr, "health-probe: no pod address")
		return "", false
	}
	return addrs[0], true
}

func check(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	port := fs.Int("port", 0, "container port")
	path := fs.String("path", "/", "request path and query")
	host := fs.String("host", "", "Host header")
	timeout := fs.Duration("timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil || *port <= 0 {
		return 2
	}
	addr, ok := podAddr()
	if !ok {
		return 1
	}
	ctx, cancel := checkContext(*timeout)
	defer cancel()
	if err := healthprobe.Check(ctx, addr, *port, *path, *host, *timeout); err != nil {
		_, _ = fmt.Fprintln(failures, err)
		return 1
	}
	return 0
}

func openPort(args []string) int {
	fs := flag.NewFlagSet("open-port", flag.ContinueOnError)
	skipList := fs.String("skip", "", "comma-separated ports that are not the service's")
	timeout := fs.Duration("timeout", 5*time.Second, "check timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var skip []int
	for field := range strings.SplitSeq(*skipList, ",") {
		if field == "" {
			continue
		}
		port, err := strconv.Atoi(field)
		if err != nil {
			return 2
		}
		skip = append(skip, port)
	}
	addr, ok := podAddr()
	if !ok {
		return 1
	}
	ctx, cancel := checkContext(*timeout)
	defer cancel()
	if _, err := healthprobe.OpenPort(ctx, addr, healthprobe.Listening(), skip, *timeout); err != nil {
		_, _ = fmt.Fprintln(failures, err)
		return 1
	}
	return 0
}

// tcp is the kubelet's TCP probe: one connection to port on the pod address.
func tcp(args []string) int {
	fs := flag.NewFlagSet("tcp", flag.ContinueOnError)
	port := fs.Int("port", 0, "container port")
	timeout := fs.Duration("timeout", 5*time.Second, "connect timeout")
	if err := fs.Parse(args); err != nil || *port <= 0 {
		return 2
	}
	addr, ok := podAddr()
	if !ok {
		return 1
	}
	ctx, cancel := checkContext(*timeout)
	defer cancel()
	if _, err := healthprobe.OpenPort(ctx, addr, []int{*port}, nil, *timeout); err != nil {
		_, _ = fmt.Fprintln(failures, err)
		return 1
	}
	return 0
}
