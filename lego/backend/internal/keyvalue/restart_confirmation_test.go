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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

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
		"listener already runs":        {func(kv *appv1alpha1.KeyValue) { kv.Status.TLSListener = true }, rule},
		"listener switched at runtime": {func(kv *appv1alpha1.KeyValue) { kv.Status.TLSRuntimeSwitch = true }, rule},
		"persisted store":              {func(kv *appv1alpha1.KeyValue) { kv.Spec.PersistenceMode = "journal-snapshot" }, rule},
		"suspended store":              {func(kv *appv1alpha1.KeyValue) { kv.Spec.Suspended = true }, rule},
		"already public":               {func(kv *appv1alpha1.KeyValue) { kv.Spec.Public = true }, append(rule, core.IPAllowListEntry{CIDRBlock: "198.51.100.0/24"})},
		"disabling external access":    {func(kv *appv1alpha1.KeyValue) { kv.Spec.Public = true }, []core.IPAllowListEntry{}},
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
		// w8/m134: a store whose pod switches TLS at runtime never restarts
		// for external access, though no listener runs while it is private.
		"private, runtime switch": {memoryOnlyStore(func(kv *appv1alpha1.KeyValue) { kv.Status.TLSRuntimeSwitch = true }), false},
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

// TestARestartFreeChangeReadsAvailable: an access flip with the TLS listener
// already running restarts nothing, so the status must not read config_restart
// while the operator catches up to it.
func TestARestartFreeChangeReadsAvailable(t *testing.T) {
	ready := func(observed int64) []metav1.Condition {
		return []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Ready", ObservedGeneration: observed}}
	}
	store := func(generation, observed int64, marked string) *appv1alpha1.KeyValue {
		kv := memoryOnlyStore(func(kv *appv1alpha1.KeyValue) { kv.Status.TLSListener = true })
		kv.Generation = generation
		kv.Status.Phase = appv1alpha1.KVPhaseReady
		kv.Status.Conditions = ready(observed)
		if marked != "" {
			kv.Annotations = map[string]string{annotationRestartFreeGeneration: marked}
		}
		return kv
	}
	for name, tc := range map[string]struct {
		kv   *appv1alpha1.KeyValue
		want string
	}{
		"marked, one generation pending":   {store(5, 4, "5"), core.DatastoreAvailable},
		"unmarked, one generation pending": {store(5, 4, ""), core.DatastoreConfigRestart},
		"marked, two generations pending":  {store(5, 3, "5"), core.DatastoreConfigRestart},
		"marked for an older generation":   {store(5, 4, "4"), core.DatastoreConfigRestart},
		"observed":                         {store(5, 5, ""), core.DatastoreAvailable},
	} {
		if got := kvStatus(tc.kv); got != tc.want {
			t.Errorf("%s: status %q, want %q", name, got, tc.want)
		}
	}

	// The write decides: allow-list and access changes with the listener
	// running are marked; a first enable or a persistence change is not.
	for name, tc := range map[string]struct {
		before *appv1alpha1.KeyValue
		patch  KeyValuePatch
		marked bool
	}{
		"empty the allow-list":     {memoryOnlyStore(func(kv *appv1alpha1.KeyValue) { kv.Spec.Public = true; kv.Status.TLSListener = true }), KeyValuePatch{IPAllowList: &[]core.IPAllowListEntry{}}, true},
		"re-enable, listener kept": {memoryOnlyStore(func(kv *appv1alpha1.KeyValue) { kv.Status.TLSListener = true }), KeyValuePatch{IPAllowList: &[]core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}}, true},
		"first enable":             {memoryOnlyStore(nil), KeyValuePatch{IPAllowList: &[]core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}}, false},
		"persistence change":       {memoryOnlyStore(func(kv *appv1alpha1.KeyValue) { kv.Status.TLSListener = true }), KeyValuePatch{PersistenceMode: new("journal_snapshot")}, false},
	} {
		tc.before.Generation = 7
		after := tc.before.DeepCopy()
		tc.patch.apply(after)
		markRestartFree(tc.before, after)
		if got := after.Annotations[annotationRestartFreeGeneration] == "8"; got != tc.marked {
			t.Errorf("%s: marked %v, want %v (%v)", name, got, tc.marked, after.Annotations)
		}
	}

	// Saving the same rules bumps no generation: marking it would let the next
	// write that does restart the store read available.
	same := memoryOnlyStore(func(kv *appv1alpha1.KeyValue) {
		kv.Spec.Public = true
		kv.Status.TLSListener = true
		kv.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.7/32"}}
	})
	same.Generation = 7
	after := same.DeepCopy()
	KeyValuePatch{IPAllowList: &[]core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}}.apply(after)
	markRestartFree(same, after)
	if marked, ok := after.Annotations[annotationRestartFreeGeneration]; ok {
		t.Errorf("an unchanged save marked generation %s restart-free", marked)
	}
}

// TestARestartFreeMarkNeverLandsOnARacingWrite: the mark names the generation
// after the save's read. Another writer that lands first with the same rules
// and a plan change takes that generation and restarts the store, so the save
// must re-read rather than mark it.
func TestARestartFreeMarkNeverLandsOnARacingWrite(t *testing.T) {
	kv := memoryOnlyStore(func(kv *appv1alpha1.KeyValue) {
		kv.Spec.Public = true
		kv.Status.TLSListener = true
	})
	kv.Generation = 7
	svc, cl := newService(kv)
	raced := false
	svc.Client = interceptor.NewClient(cl.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if !raced {
				raced = true
				current := getKeyValueCR(t, c, obj.GetName())
				current.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.7/32"}}
				current.Spec.Plan = "starter"
				current.Generation = 8
				if err := c.Update(ctx, current); err != nil {
					return err
				}
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	})
	rule := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.7/32"}}
	if _, err := svc.UpdateKeyValue(context.Background(), "red-cache", KeyValuePatch{IPAllowList: &rule}); err != nil {
		t.Fatal(err)
	}
	if marked, ok := getKeyValueCR(t, cl, "red-cache").Annotations[annotationRestartFreeGeneration]; ok {
		t.Fatalf("the save marked generation %s restart-free after a racing plan change", marked)
	}
}
