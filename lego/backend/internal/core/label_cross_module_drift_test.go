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

import "testing"

// TestCrossModuleLabelKeysMatchOperator pins the label keys bex-api shares
// with the operator by VALUE. The backend cannot be imported by the operator
// (import DAG: operator→types←backend), so these spellings are hand-synced
// with lego/operator (see the counterpart test there,
// TestCrossModuleLabelKeysMatchBackend in internal/controller). Changing a key
// here without changing it there (or vice versa) silently breaks pod
// selection, NetworkPolicy boundaries, and log/metric lookups across the App
// CR boundary — so a change here must update the operator constant, its test,
// and the rollout note in the same commit.
func TestCrossModuleLabelKeysMatchOperator(t *testing.T) {
	want := map[string]string{
		"PodLabelApp":           "app.bex.co/app",
		"PodLabelRevision":      "app.bex.co/revision",
		"PodLabelComponent":     "app.bex.co/component",
		"LabelAppID":            "bex.co/app-id",
		"LabelWorkspace":        "app.bex.co/workspace",
		"LabelEnvironment":      "app.bex.co/environment-id",
		"LabelNetworkIsolation": "app.bex.co/network-isolation",
	}
	got := map[string]string{
		"PodLabelApp":           PodLabelApp,
		"PodLabelRevision":      PodLabelRevision,
		"PodLabelComponent":     PodLabelComponent,
		"LabelAppID":            LabelAppID,
		"LabelWorkspace":        LabelWorkspace,
		"LabelEnvironment":      LabelEnvironment,
		"LabelNetworkIsolation": LabelNetworkIsolation,
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s = %q, want %q (hand-synced with lego/operator — update both sides)", key, got[key], w)
		}
	}
}
