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

package core

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// featureSentinel stands in for a sentinel declared by a FEATURE package
// (projects, environments, members, jobs, blueprints). Core cannot import one
// to test against — it is a leaf — so the test declares its own the same way a
// feature package does. This is the case that used to be impossible: before
// ErrUnavailable, a sentinel core could not name fell through WriteErr to 500.
var featureSentinel = Unavailable("test feature store not configured")

func TestUnavailablePreservesTheSentinelMessage(t *testing.T) {
	// The wire text is the contract — callers and the CLI read `.message`, so
	// the marker must not prefix or decorate it.
	if got := featureSentinel.Error(); got != "test feature store not configured" {
		t.Fatalf("Error() = %q, want the message verbatim", got)
	}
	if got := ErrLogsUnavailable.Error(); got != "logs source not configured" {
		t.Fatalf("ErrLogsUnavailable = %q, want the message verbatim", got)
	}
}

func TestUnavailableSentinelsCarryTheMarkerButStayDistinct(t *testing.T) {
	if !errors.Is(featureSentinel, ErrUnavailable) {
		t.Error("a feature-declared sentinel does not carry the ErrUnavailable marker")
	}
	if !errors.Is(ErrLogsUnavailable, ErrUnavailable) {
		t.Error("a core sentinel does not carry the ErrUnavailable marker")
	}
	// Sharing a marker must not collapse the family into one error: a verb
	// asking "is this specifically the log store?" still needs a precise answer.
	if errors.Is(ErrLogsUnavailable, ErrMetricsUnavailable) {
		t.Error("two distinct unavailable sentinels compare Is-equal")
	}
	if errors.Is(featureSentinel, ErrLogsUnavailable) {
		t.Error("a feature sentinel compares Is-equal to a core one")
	}
	if !errors.Is(ErrLogsUnavailable, ErrLogsUnavailable) {
		t.Error("a sentinel is not Is-equal to itself")
	}
	// The marker must not leak into unrelated families.
	if errors.Is(ErrBadRequest, ErrUnavailable) || errors.Is(ErrNotFound, ErrUnavailable) {
		t.Error("an unrelated sentinel carries the ErrUnavailable marker")
	}
}

func TestWriteErrMapsEveryUnavailableSentinelTo503(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "core sentinel", err: ErrLogsUnavailable},
		{name: "feature-declared sentinel", err: featureSentinel},
		{name: "wrapped with context", err: fmt.Errorf("listing environments: %w", featureSentinel)},
		{name: "the bare marker", err: ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteErr(w, tc.err)
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", w.Code)
			}
			// 503 must speak the same Render-shaped envelope as every other
			// status, since the official CLI reads `.message`.
			if got, want := w.Body.String(), `"id":"unavailable"`; !strings.Contains(got, want) {
				t.Fatalf("body = %s, want it to contain %s", got, want)
			}
			if got := w.Body.String(); !strings.Contains(got, `"message":"`+tc.err.Error()+`"`) {
				t.Fatalf("body = %s, want message %q", got, tc.err.Error())
			}
		})
	}
}

// TestNoUnavailableSentinelBypassesTheMarker is the drift guard. Declaring a
// sentinel with errors.New instead of core.Unavailable compiles and reads
// fine, but silently answers 500 where the feature's docs promise 503 — which
// is exactly what members, jobs, and blueprints did before this marker existed.
// The declaration site is the only place that can go wrong, so scan for it.
func TestNoUnavailableSentinelBypassesTheMarker(t *testing.T) {
	// The whole backend module: internal/core -> backend/, so cmd/ is covered
	// too, not just the feature packages under internal/.
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// Err\w+Unavailable, not Err\w*Unavailable: the marker ErrUnavailable is
	// itself a plain errors.New and is the one legitimate exception.
	bypass := regexp.MustCompile(`(?m)^\s*(?:var\s+)?Err\w+Unavailable\s+=\s+(?:errors\.New|Err)\(`)
	var offenders []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range bypass.FindAllString(string(source), -1) {
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, fmt.Sprintf("%s: %s", rel, strings.TrimSpace(match)))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("these sentinels bypass the ErrUnavailable marker and will answer 500 instead of 503 —\n"+
			"declare them with core.Unavailable(\"…\") instead of errors.New(\"…\"):\n\t%s",
			strings.Join(offenders, "\n\t"))
	}
}
