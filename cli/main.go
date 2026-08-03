// Command bex runs the upstream Render CLI against a Bex control plane.
//
// The launcher owns only process configuration. Command parsing, output, and
// API client behavior remain in github.com/render-oss/cli so that a Bex CLI
// upgrade is an explicit upstream dependency update rather than a fork.
package main

import (
	"fmt"
	"os"

	"github.com/bex-co/bex/cli/internal/bridge"
	"github.com/render-oss/cli/cmd"
)

func main() {
	if err := bridge.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "bex: configure upstream CLI: %v\n", err)
		os.Exit(1)
	}
	os.Exit(cmd.Execute())
}
