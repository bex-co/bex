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

package api

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// sha256Hex builds fixture digests for the corruption cases below.
func sha256Hex(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

// TestRenderMCPPinLoads is the counterpart of the REST spec's integrity check:
// the embedded capture must parse and match its pinned digest, so a hand-edit
// cannot silently weaken every parity assertion built on it.
func TestRenderMCPPinLoads(t *testing.T) {
	c, err := loadRenderMCPContract()
	if err != nil {
		t.Fatalf("load pin: %v", err)
	}
	if len(c.Tools) == 0 {
		t.Fatal("pin has no tools")
	}
	if c.Source.Repo != "github.com/render-oss/render-mcp-server" {
		t.Errorf("unexpected upstream repo %q", c.Source.Repo)
	}
	if len(c.Source.Commit) != 40 {
		t.Errorf("upstream commit %q is not a full sha", c.Source.Commit)
	}
	// A tool every version of the official server has registered; if this is
	// missing the capture ran against the wrong binary.
	if _, ok := c.Tool("list_services"); !ok {
		t.Error("pin is missing list_services — capture likely failed")
	}
}

func TestRenderMCPPinRejectsCorruption(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		hash string
		want string
	}{
		{
			name: "wrong digest",
			data: renderMCPToolsSource,
			hash: strings.Repeat("0", 64),
			want: "integrity mismatch",
		},
		{
			name: "truncated file",
			data: renderMCPToolsSource[:len(renderMCPToolsSource)/2],
			hash: sha256Hex(renderMCPToolsSource[:len(renderMCPToolsSource)/2]),
			want: "parse embedded",
		},
		{
			name: "malformed json",
			data: []byte("{not json"),
			hash: sha256Hex([]byte("{not json")),
			want: "parse embedded",
		},
		{
			name: "no tools",
			data: []byte(`{"source":{"commit":"` + strings.Repeat("a", 40) + `"},"tools":[]}`),
			hash: sha256Hex([]byte(`{"source":{"commit":"` + strings.Repeat("a", 40) + `"},"tools":[]}`)),
			want: "no tools",
		},
		{
			name: "no upstream commit",
			data: []byte(`{"source":{},"tools":[{"name":"x"}]}`),
			hash: sha256Hex([]byte(`{"source":{},"tools":[{"name":"x"}]}`)),
			want: "no upstream commit",
		},
		{
			name: "duplicate tool",
			data: []byte(`{"source":{"commit":"` + strings.Repeat("a", 40) + `"},"tools":[{"name":"x"},{"name":"x"}]}`),
			hash: sha256Hex([]byte(`{"source":{"commit":"` + strings.Repeat("a", 40) + `"},"tools":[{"name":"x"},{"name":"x"}]}`)),
			want: "twice",
		},
		{
			name: "unnamed tool",
			data: []byte(`{"source":{"commit":"` + strings.Repeat("a", 40) + `"},"tools":[{"name":""}]}`),
			hash: sha256Hex([]byte(`{"source":{"commit":"` + strings.Repeat("a", 40) + `"},"tools":[{"name":""}]}`)),
			want: "unnamed tool",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadRenderMCPContractData(tc.data, tc.hash)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}

	// The good pin still loads after all the corruption cases — no shared state.
	if _, err := loadRenderMCPContract(); err != nil {
		t.Fatalf("pin no longer loads: %v", err)
	}
}

// TestRenderMCPPinMismatchNamesTheFix keeps the failure actionable: whoever
// trips this is refreshing the pin and needs to be told how.
func TestRenderMCPPinMismatchNamesTheFix(t *testing.T) {
	_, err := loadRenderMCPContractData(renderMCPToolsSource, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("want error")
	}
	for _, want := range []string{"render-mcp-capture.py", "renderMCPToolsSHA256"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("integrity error does not mention %q: %v", want, err)
		}
	}
}
