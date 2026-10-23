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
// and exits 0 for a 2xx/3xx answer.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/healthprobe"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: health-probe install DEST | check --port N --path P [--host H] [--timeout D]")
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
	case "check":
		os.Exit(check(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "health-probe: unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
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
	// The pod address, the one the HTTP probe dialed: an app listening only on
	// localhost is as unreachable here as it is from the edge.
	addrs := healthprobe.PodAddrs()
	if len(addrs) == 0 {
		fmt.Fprintln(os.Stderr, "health-probe: no pod address")
		return 1
	}
	if err := healthprobe.Check(context.Background(), addrs[0], *port, *path, *host, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "health-probe:", err)
		return 1
	}
	return 0
}
