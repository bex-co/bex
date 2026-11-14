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
	"encoding/json"
	"flag"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// The manager logs in zap's production mode: JSON, info level, stack traces on
// errors only, sampling on. Development mode (the kubebuilder scaffold's
// default) logs at debug, prints a stack trace at every warning, turns DPanic
// into a panic and never samples.
func TestTheManagerLogsInProductionModeByDefault(t *testing.T) {
	opts := logOptions()
	var out bytes.Buffer
	opts.DestWriter = &out
	logger := zap.New(zap.UseFlagOptions(&opts))

	logger.V(1).Info("debug detail")
	logger.Info("started", "port", 9443)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1 (debug is off by default): %q", len(lines), out.String())
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("log line is not JSON: %q (%v)", lines[0], err)
	}
	if record["msg"] != "started" || record["port"] != float64(9443) {
		t.Fatalf("record = %v", record)
	}
}

// A developer can still ask for development mode with the standard flag.
func TestZapDevelStillTurnsDevelopmentModeOn(t *testing.T) {
	opts := logOptions()
	fs := flag.NewFlagSet("manager", flag.ContinueOnError)
	opts.BindFlags(fs)
	if err := fs.Parse([]string{"--zap-devel=true"}); err != nil {
		t.Fatal(err)
	}
	if !opts.Development {
		t.Fatal("--zap-devel=true did not enable development mode")
	}
}
