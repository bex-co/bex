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
	"net/url"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m233: turning external access on for the first time adds the TLS
// listener, which restarts the store; with persistence off that deletes every
// key. Every writer refuses it without the restart phrase, and accepts it with.

var restartPhrase = ProtectedConfirmation("clear", "cache")

func memoryOnlyStore(mutate func(*appv1alpha1.KeyValue)) *appv1alpha1.KeyValue {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-cache", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Name: "cache", Plan: "free", PersistenceMode: "off"},
	}
	if mutate != nil {
		mutate(kv)
	}
	return kv
}

// mcpUpdate calls update_key_value and returns the tool error text, if any.
func mcpUpdate(t *testing.T, svc *Service, args map[string]any) string {
	t.Helper()
	cs := kvMCPSession(t, svc)
	defer cs.Close()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "update_key_value", Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		return ""
	}
	b, _ := json.Marshal(res.Content)
	return string(b)
}

func TestFirstExternalEnableOfAMemoryOnlyStoreNeedsTheRestartPhrase(t *testing.T) {
	rule := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}
	// Each writer returns its refusal as the client sees it: REST's body, and
	// GraphQL's message plus its extensions code. MCP's tool error is the
	// message alone, so only the phrase is asserted there.
	writers := map[string]func(t *testing.T, svc *Service, confirm string) string{
		"PATCH": func(t *testing.T, svc *Service, confirm string) string {
			w := serveREST(svc, "PATCH", "/v1/key-value/red-cache?confirm="+url.QueryEscape(confirm), `{"ipAllowList":[{"cidrBlock":"203.0.113.7/32"}]}`)
			if w.Code == 200 {
				return ""
			}
			return w.Body.String()
		},
		"PUT": func(t *testing.T, svc *Service, confirm string) string {
			w := serveREST(svc, "PUT", "/v1/key-value/red-cache/ip-allow-list?confirm="+url.QueryEscape(confirm), `{"cidrs":["203.0.113.7/32"]}`)
			if w.Code == 200 {
				return ""
			}
			return w.Body.String()
		},
		"GraphQL allowlist": func(t *testing.T, svc *Service, confirm string) string {
			schema, err := kvGQLSchema(svc)
			if err != nil {
				t.Fatal(err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
				RequestString:  `mutation($c: String) { setKeyValueIpAllowList(id:"red-cache", cidrs:["203.0.113.7/32"], confirm:$c) { public } }`,
				VariableValues: map[string]any{"c": confirm}})
			if len(res.Errors) == 0 {
				return ""
			}
			return fmt.Sprint(res.Errors[0].Message, " ", res.Errors[0].Extensions["code"])
		},
		"GraphQL public": func(t *testing.T, svc *Service, confirm string) string {
			schema, err := kvGQLSchema(svc)
			if err != nil {
				t.Fatal(err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
				RequestString:  `mutation($c: String) { setKeyValuePublic(id:"red-cache", public:true, confirm:$c) { public } }`,
				VariableValues: map[string]any{"c": confirm}})
			if len(res.Errors) == 0 {
				return ""
			}
			return fmt.Sprint(res.Errors[0].Message, " ", res.Errors[0].Extensions["code"])
		},
		"MCP": func(t *testing.T, svc *Service, confirm string) string {
			return mcpUpdate(t, svc, map[string]any{"keyValueId": "red-cache", "ipAllowList": rule, "confirm": confirm})
		},
	}
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			svc, cl := newService(memoryOnlyStore(nil))
			refusal := write(t, svc, "")
			if !strings.Contains(refusal, restartPhrase) || (name != "MCP" && !strings.Contains(refusal, "KEY_VALUE_RESTART_CONFIRMATION_REQUIRED")) {
				t.Fatalf("unconfirmed first enable = %q, want the coded refusal naming %q", refusal, restartPhrase)
			}
			if stored := getKeyValueCR(t, cl, "red-cache"); stored.Spec.Public || len(stored.Spec.IPAllowList) != 0 {
				t.Fatalf("a refused enable wrote public=%v rules=%v", stored.Spec.Public, stored.Spec.IPAllowList)
			}
			if refusal := write(t, svc, restartPhrase); refusal != "" {
				t.Fatalf("confirmed first enable refused: %s", refusal)
			}
			if !getKeyValueCR(t, cl, "red-cache").Spec.Public {
				t.Fatal("the confirmed enable did not publish the store")
			}
		})
	}
}

func TestExternalAccessChangesThatRestartNothingNeedNoPhrase(t *testing.T) {
	rule := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}
	for name, tc := range map[string]struct {
		store   func(*appv1alpha1.KeyValue)
		entries []core.IPAllowListEntry
	}{
		"listener already runs":     {func(kv *appv1alpha1.KeyValue) { kv.Status.TLSListener = true }, rule},
		"persisted store":           {func(kv *appv1alpha1.KeyValue) { kv.Spec.PersistenceMode = "journal-snapshot" }, rule},
		"suspended store":           {func(kv *appv1alpha1.KeyValue) { kv.Spec.Suspended = true }, rule},
		"already public":            {func(kv *appv1alpha1.KeyValue) { kv.Spec.Public = true }, append(rule, core.IPAllowListEntry{CIDRBlock: "198.51.100.0/24"})},
		"disabling external access": {func(kv *appv1alpha1.KeyValue) { kv.Spec.Public = true }, []core.IPAllowListEntry{}},
	} {
		t.Run(name, func(t *testing.T) {
			svc, _ := newService(memoryOnlyStore(tc.store))
			if _, err := svc.SetIPAllowList(context.Background(), "red-cache", tc.entries); err != nil {
				t.Fatalf("SetIPAllowList = %v, want no confirmation needed", err)
			}
		})
	}
}

func TestExternalAccessRestartsOnlyBeforeTheListenerRuns(t *testing.T) {
	for name, tc := range map[string]struct {
		store *appv1alpha1.KeyValue
		want  bool
	}{
		"private, never listened": {memoryOnlyStore(nil), true},
		"public":                  {memoryOnlyStore(func(kv *appv1alpha1.KeyValue) { kv.Spec.Public = true; kv.Status.TLSListener = true }), false},
		"private, listener kept":  {memoryOnlyStore(func(kv *appv1alpha1.KeyValue) { kv.Status.TLSListener = true }), false},
		"suspended":               {memoryOnlyStore(func(kv *appv1alpha1.KeyValue) { kv.Spec.Suspended = true }), false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := kvView(tc.store).ExternalAccessRestarts; got != tc.want {
				t.Fatalf("ExternalAccessRestarts = %v, want %v", got, tc.want)
			}
		})
	}
	// The refusal is a coded bad request carrying the phrase as a param.
	err := requireRestartConfirmation(context.Background(), memoryOnlyStore(nil), KeyValuePatch{IPAllowList: &[]core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "KEY_VALUE_RESTART_CONFIRMATION_REQUIRED" || coded.Params["confirm"] != restartPhrase || !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("refusal = %#v", err)
	}
}
