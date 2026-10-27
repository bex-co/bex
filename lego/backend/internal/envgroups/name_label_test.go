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

package envgroups

import (
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// An env group name follows the label rule on create, rename and Blueprint
// apply: no control characters, at most 100 code points (w4/419).
func TestEnvGroupNamesFollowTheLabelRule(t *testing.T) {
	ctx := testCtx()
	for name, code := range map[string]string{
		"line1\nline2":           "DISPLAY_NAME_CONTROL_CHARACTER",
		"tab\tsep":               "DISPLAY_NAME_CONTROL_CHARACTER",
		"bell\u0007end":          "DISPLAY_NAME_CONTROL_CHARACTER",
		strings.Repeat("g", 101): "DISPLAY_NAME_TOO_LONG",
	} {
		svc := newService(newFakeStore())
		group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "ok"})
		if err != nil {
			t.Fatal(err)
		}
		_, createErr := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: name})
		_, renameErr := svc.RenameEnvGroup(ctx, group.ID, name)
		applyErr := svc.ApplyEnvGroup(ctx, name, map[string]string{"A": "1"}, nil)
		for verb, err := range map[string]error{"create": createErr, "rename": renameErr, "apply": applyErr} {
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != code {
				t.Errorf("%s %q = %v, want %s", verb, name, err, code)
			}
		}
	}
	svc := newService(newFakeStore())
	if g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: " pad "}); err != nil || g.Name != "pad" {
		t.Fatalf("padded name = %+v, %v; want trimmed and accepted", g, err)
	}
	if _, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: strings.Repeat("é", 100)}); err != nil {
		t.Fatalf("a 100-code-point name = %v, want accepted", err)
	}
}

// An env group var no process could start with is refused on create and on
// a per-key write (w4/422).
func TestEnvGroupRefusesAnEntryNoProcessCanStartWith(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	over := strings.Repeat("v", core.MaxEnvEntryBytes)
	var coded *core.CodedError
	if _, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "big", EnvVars: []CreateEnvVarInput{{Key: "BIG", Value: over}}}); !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_VARIABLE_TOO_LARGE" {
		t.Fatalf("create = %v, want ENVIRONMENT_VARIABLE_TOO_LARGE", err)
	}
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "small"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetEnvGroupVar(ctx, group.ID, "BIG", over); !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_VARIABLE_TOO_LARGE" {
		t.Fatalf("set = %v, want ENVIRONMENT_VARIABLE_TOO_LARGE", err)
	}
}
