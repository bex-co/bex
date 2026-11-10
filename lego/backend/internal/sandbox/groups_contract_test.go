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

package sandbox

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// w22/m311: the group routes emit the shape the pinned Render CLI is proven to
// read (lego/cli/sandbox_groups_contract_test.go runs the pinned commands against
// this same golden). Keys, JSON types and the fixed values must match; ids and
// timestamps are fixtures.
func TestGroupRoutesEmitTheCLIContractGolden(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "sandbox-groups-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Create struct {
			Status   int            `json:"status"`
			Response map[string]any `json:"response"`
		} `json:"create"`
		List struct {
			Response []map[string]any `json:"response"`
		} `json:"list"`
		SandboxCreate struct {
			Response map[string]any `json:"response"`
		} `json:"sandboxCreate"`
		Errors map[string]struct {
			Status int    `json:"status"`
			Code   string `json:"code"`
			ID     string `json:"id"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}

	f := newGroupFixture(t)
	f.svc.MaxGroups = 1

	code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"agents"}`, f.ws))
	if code != golden.Create.Status {
		t.Fatalf("create = %d %s, want %d", code, body, golden.Create.Status)
	}
	var created map[string]any
	_ = json.Unmarshal([]byte(body), &created)
	sameShape(t, "create", created, golden.Create.Response)
	for _, key := range []string{"name", "region", "isDefault", "environmentId", "concurrencyLimit"} {
		if created[key] != golden.Create.Response[key] {
			t.Errorf("create %s = %v, want the golden's %v", key, created[key], golden.Create.Response[key])
		}
	}

	_, body = f.do(t, http.MethodGet, "/v1/sandbox-groups?ownerId="+f.ws, "")
	var listed []map[string]any
	if err := json.Unmarshal([]byte(body), &listed); err != nil || len(listed) != len(golden.List.Response) {
		t.Fatalf("list = %s (%v), want %d item(s)", body, err, len(golden.List.Response))
	}
	sameShape(t, "list item", listed[0], golden.List.Response[0])
	sameShape(t, "list item sandboxGroup", listed[0]["sandboxGroup"].(map[string]any), golden.List.Response[0]["sandboxGroup"].(map[string]any))

	sb := f.mustCreate(t, created["id"].(string))
	raw2, _ := json.Marshal(sb)
	var sandbox map[string]any
	_ = json.Unmarshal(raw2, &sandbox)
	for key := range golden.SandboxCreate.Response {
		if _, ok := sandbox[key]; !ok {
			t.Errorf("a sandbox created in a group lacks %q (the golden has it)", key)
		}
	}
	if sandbox["sandboxGroupId"] != created["id"] {
		t.Errorf("sandbox sandboxGroupId = %v, want %v", sandbox["sandboxGroupId"], created["id"])
	}

	// The two refusals the pinned CLI documents: 409 for a taken name, 400 at the
	// group cap. Each is bex's error envelope with its code.
	for name, call := range map[string]func() (int, string){
		"nameTaken": func() (int, string) {
			f.svc.MaxGroups = 5
			return f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"agents"}`, f.ws))
		},
		"groupLimit": func() (int, string) {
			f.svc.MaxGroups = 1
			return f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"another"}`, f.ws))
		},
	} {
		want := golden.Errors[name]
		code, body := call()
		var got map[string]any
		_ = json.Unmarshal([]byte(body), &got)
		if code != want.Status || got["code"] != want.Code || got["id"] != want.ID {
			t.Errorf("%s = %d %s, want %d %s (%s)", name, code, body, want.Status, want.Code, want.ID)
		}
		for _, key := range []string{"error", "message"} {
			if s, _ := got[key].(string); s == "" {
				t.Errorf("%s has no %q: %s", name, key, body)
			}
		}
	}
}

// sameShape fails when two objects differ in their keys or in a value's JSON
// type; null in the golden matches null only.
func sameShape(t *testing.T, what string, got, want map[string]any) {
	t.Helper()
	keys := func(m map[string]any) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	if !slices.Equal(keys(got), keys(want)) {
		t.Errorf("%s keys = %v, want %v", what, keys(got), keys(want))
	}
	for key, w := range want {
		if fmt.Sprintf("%T", got[key]) != fmt.Sprintf("%T", w) {
			t.Errorf("%s.%s is a %T, want a %T", what, key, got[key], w)
		}
	}
}
