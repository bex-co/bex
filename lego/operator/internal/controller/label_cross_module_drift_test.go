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

package controller

import (
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/execution"
)

// TestCrossModuleLabelKeysMatchBackend pins the label keys the operator shares
// with bex-api by VALUE. The operator cannot import the backend (import DAG:
// operator→types←backend), so these spellings are hand-synced with
// lego/backend/internal/core (see the counterpart test there,
// TestCrossModuleLabelKeysMatchOperator). Changing a key here without changing
// it there (or vice versa) silently breaks pod selection, NetworkPolicy
// boundaries, and log/metric lookups across the App CR boundary — so a change
// here must update the backend constant, its test, and the rollout note in the
// same commit.
func TestCrossModuleLabelKeysMatchBackend(t *testing.T) {
	want := map[string]string{
		// labelApp / labelWorkspace are aliases of the execution package (one
		// operator-side definition); the literals below pin the cross-module
		// contract against backend core.PodLabelApp / core.LabelWorkspace.
		"labelApp":         "app.bex.co/app",
		"labelWorkspace":   "app.bex.co/workspace",
		"labelEnvironment": "app.bex.co/environment-id",
		// Single-definition controller consts, hand-synced with backend core.
		"labelAppID":            "bex.co/app-id",
		"labelRevision":         "app.bex.co/revision",
		"labelNetworkIsolation": "app.bex.co/network-isolation",
		// execution package consts, hand-synced with backend core.
		"execution.LabelComponent": "app.bex.co/component",
	}
	got := map[string]string{
		"labelApp":                 labelApp,
		"labelWorkspace":           labelWorkspace,
		"labelEnvironment":         labelEnvironment,
		"labelAppID":               labelAppID,
		"labelRevision":            labelRevision,
		"labelNetworkIsolation":    labelNetworkIsolation,
		"execution.LabelComponent": execution.LabelComponent,
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s = %q, want %q (hand-synced with lego/backend/internal/core — update both sides)", key, got[key], w)
		}
	}
	// The in-operator aliases must keep pointing at the execution package: if
	// someone re-forks the literal, the cross-module pin above still passes
	// while the operator drifts internally.
	if labelApp != execution.LabelApp {
		t.Errorf("labelApp = %q, want execution.LabelApp %q", labelApp, execution.LabelApp)
	}
	if labelWorkspace != execution.LabelWorkspace {
		t.Errorf("labelWorkspace = %q, want execution.LabelWorkspace %q", labelWorkspace, execution.LabelWorkspace)
	}
	if labelEnvironment != execution.LabelEnvironment {
		t.Errorf("labelEnvironment = %q, want execution.LabelEnvironment %q", labelEnvironment, execution.LabelEnvironment)
	}
}
