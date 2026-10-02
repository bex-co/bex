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
package keyvalue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Array-only round trips missed the original defect: explicit clears must also
// withdraw the external route through every writer, including dashboard setters.
func TestAllowListAccessAcrossWriters(t *testing.T) {
	for _, adapter := range []string{"PATCH", "PUT", "GraphQL", "MCP"} {
		t.Run(adapter, func(t *testing.T) {
			resource := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: "red-access", Namespace: "default"},
				Spec:       appv1alpha1.KeyValueSpec{Name: "networking", Plan: "free", Public: true},
			}
			svc, cl := newService(resource)
			schema, err := kvGQLSchema(svc)
			if err != nil {
				t.Fatal(err)
			}
			call, cleanup := kvMCPClient(t, svc)
			defer cleanup()
			for _, entries := range [][]core.IPAllowListEntry{{}, {{CIDRBlock: "203.0.113.7/32", Description: "v4"}, {CIDRBlock: "2001:db8::7/128", Description: "v6"}}, {}} {
				switch adapter {
				case "PATCH", "PUT":
					body := map[string]any{"ipAllowList": entries}
					if adapter == "PUT" {
						cidrs := core.AllowListCIDRs(entries)
						body = map[string]any{"cidrs": cidrs}
					}
					payload, err := json.Marshal(body)
					if err != nil {
						t.Fatal(err)
					}
					path := "/v1/key-value/red-access"
					if adapter == "PUT" {
						path += "/ip-allow-list"
					}
					w := serveREST(svc, adapter, path, string(payload))
					if w.Code != 200 {
						t.Fatalf("%s: %d %s", adapter, w.Code, w.Body.String())
					}
				case "GraphQL":
					cidrs := make([]string, len(entries))
					for i, e := range entries {
						cidrs[i] = e.CIDRBlock
					}
					result := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
						RequestString:  `mutation($rules:[String]) { setKeyValueIpAllowList(id:"red-access", cidrs:$rules) { public } }`,
						VariableValues: map[string]any{"rules": cidrs}})
					if len(result.Errors) > 0 {
						t.Fatal(result.Errors)
					}
					got := result.Data.(map[string]any)["setKeyValueIpAllowList"].(map[string]any)
					if got["public"] != (len(entries) > 0) {
						t.Fatalf("GraphQL public readback=%v", got)
					}
				case "MCP":
					got := call("update_key_value", map[string]any{"keyValueId": "red-access", "ipAllowList": entries})
					if got["public"] != (len(entries) > 0) {
						t.Fatalf("MCP public readback=%v", got)
					}
				}
				var stored appv1alpha1.KeyValue
				if err := cl.Get(context.Background(), client.ObjectKeyFromObject(resource), &stored); err != nil {
					t.Fatal(err)
				}
				if stored.Spec.Public != (len(entries) > 0) || len(stored.Spec.IPAllowList) != len(entries) {
					t.Fatalf("%s left external access=%v rules=%v after %v", adapter, stored.Spec.Public, stored.Spec.IPAllowList, entries)
				}
			}
		})
	}
}

func TestAllowListIntentMatrixAndPreview(t *testing.T) {
	empty := []core.IPAllowListEntry{}
	rules := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}
	on, off := true, false
	cases := []struct {
		name  string
		patch KeyValuePatch
		want  *bool
		count int
	}{
		{"omitted", KeyValuePatch{}, nil, -1},
		{"clear", KeyValuePatch{IPAllowList: &empty}, &off, 0},
		{"rules", KeyValuePatch{IPAllowList: &rules}, &on, 1},
		{"explicit open", KeyValuePatch{IPAllowList: &empty, Public: &on}, &on, 0},
		{"explicit private", KeyValuePatch{IPAllowList: &rules, Public: &off}, &off, 1},
		{"publish only", KeyValuePatch{Public: &on}, &on, -1},
		{"withdraw only", KeyValuePatch{Public: &off}, &off, -1},
	}
	for _, initialPublic := range []bool{false, true} {
		for _, initialRules := range [][]appv1alpha1.IPAllowEntry{nil, {{CIDR: "192.0.2.1/32", Description: "old"}}} {
			for _, tc := range cases {
				t.Run(fmt.Sprintf("public=%v/rules=%d/%s", initialPublic, len(initialRules), tc.name), func(t *testing.T) {
					original := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-access", Namespace: "default"}, Spec: appv1alpha1.KeyValueSpec{Name: "networking", Plan: "free", Public: initialPublic, IPAllowList: initialRules, EnvironmentIPAllowList: []string{"203.0.113.0/24"}}}
					svc, cl := newService(original)
					expected := initialPublic
					if tc.want != nil {
						expected = *tc.want
					}
					preview, err := svc.PreviewUpdateKeyValue(context.Background(), original.Name, tc.patch)
					if err != nil || preview.Public != expected {
						t.Fatalf("preview=%+v err=%v", preview, err)
					}
					var stored appv1alpha1.KeyValue
					if err := cl.Get(context.Background(), client.ObjectKeyFromObject(original), &stored); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(stored.Spec, original.Spec) {
						t.Fatal("preview mutated resource")
					}
					view, err := svc.UpdateKeyValue(context.Background(), original.Name, tc.patch)
					if err != nil || view.Public != expected {
						t.Fatalf("update=%+v err=%v", view, err)
					}
					if err := cl.Get(context.Background(), client.ObjectKeyFromObject(original), &stored); err != nil {
						t.Fatal(err)
					}
					if stored.Spec.Public != expected || !reflect.DeepEqual(stored.Spec.EnvironmentIPAllowList, original.Spec.EnvironmentIPAllowList) {
						t.Fatalf("wrong intent: %+v", stored.Spec)
					}
					if tc.count >= 0 && len(stored.Spec.IPAllowList) != tc.count {
						t.Fatalf("rules=%v", stored.Spec.IPAllowList)
					}
					if tc.count < 0 && !reflect.DeepEqual(stored.Spec.IPAllowList, initialRules) {
						t.Fatal("omitted rules changed")
					}
				})
			}
		}
	}
}

func TestRejectedAllowListPreservesAccess(t *testing.T) {
	for _, setter := range []string{"update", "dedicated"} {
		for _, failure := range []string{"invalid", "unauthorized"} {
			t.Run(setter+"/"+failure, func(t *testing.T) {
				original := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-access", Namespace: "default", Labels: map[string]string{core.LabelTenant: "tea-owner"}}, Spec: appv1alpha1.KeyValueSpec{Name: "networking", Plan: "free", Public: true, IPAllowList: []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.7/32"}}}}
				svc, cl := newService(original)
				rules := []core.IPAllowListEntry{{CIDRBlock: "invalid"}}
				want := core.ErrBadRequest
				if failure == "unauthorized" {
					svc.Authz = &fakeChecker{allow: false}
					rules = []core.IPAllowListEntry{}
					want = core.ErrForbidden
				}
				var err error
				if setter == "dedicated" {
					_, err = svc.SetIPAllowList(ctxAs("user-a"), original.Name, rules)
				} else {
					_, err = svc.UpdateKeyValue(ctxAs("user-a"), original.Name, KeyValuePatch{IPAllowList: &rules})
				}
				if !errors.Is(err, want) {
					t.Fatalf("error=%v want=%v", err, want)
				}
				var stored appv1alpha1.KeyValue
				if err := cl.Get(context.Background(), client.ObjectKeyFromObject(original), &stored); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(stored.Spec, original.Spec) {
					t.Fatal("rejected mutation changed access intent")
				}
			})
		}
	}
}
