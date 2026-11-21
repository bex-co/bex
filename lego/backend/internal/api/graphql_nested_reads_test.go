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
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestGraphQLNestedReadsAreBounded: the env-var, secret-file and outbound-IP
// fields a Service nests each read the Kubernetes API once per service they
// are selected on, which the cost gate cannot see. On a workspace at the
// paid plan's 100 services, this 2 KB document made 12,000 Kubernetes API
// calls and ran 3 seconds against an in-memory fake API server; past
// gqlMaxNestedReads a document's reads are now refused without reaching the
// reader.
func TestGraphQLNestedReadsAreBounded(t *testing.T) {
	var calls atomic.Int64
	var objs []client.Object
	for i := range 100 {
		name := fmt.Sprintf("web%03d", i)
		objs = append(objs, sampleApp(name), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-env", Namespace: "default"},
			Data:       map[string][]byte{"A": []byte("1")},
		})
	}
	counted := interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			calls.Add(1)
			return c.Get(ctx, key, obj, opts...)
		},
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			calls.Add(1)
			return c.List(ctx, list, opts...)
		},
	}
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(rendertest.Stamped(objs...)...).WithInterceptorFuncs(counted).Build()
	srv := NewServer(&core.Base{Client: cl, Namespace: "default"}, Deps{})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema

	nested := []string{`envVarKeys { key }`, `envVar(key: "A") { key }`, `secretFileNames { name }`, `secretFile(name: "f") { name }`, `outboundIps { ips }`}
	// Each field 11 times, with the 50 aliases the cost gate allows.
	var worst strings.Builder
	for f, field := range nested {
		for i := range gqlMaxAliasesPerField + 1 {
			if i > 0 {
				fmt.Fprintf(&worst, "f%d_%d: ", f, i)
			}
			worst.WriteString(field + " ")
		}
	}
	run := func(doc string) (*graphql.Result, int64, time.Duration) {
		t.Helper()
		calls.Store(0)
		var result *graphql.Result
		start := time.Now()
		finishesWithin(t, doc[:40], func() {
			_, _, _, result = srv.executeGraphQL(context.Background(), graphqlRequestBody{Query: doc}, &gqlBudget{})
		})
		return result, calls.Load(), time.Since(start)
	}
	limited := func(r *graphql.Result) int {
		n := 0
		for _, e := range r.Errors {
			if e.Extensions["code"] == errNestedReads.Code {
				n++
			}
		}
		return n
	}

	doc := "{ services(limit: 100) { id " + worst.String() + "} }"
	if err := validateGraphQLComplexity(doc); err != nil {
		t.Fatalf("the cost gate refuses the document (%v); it no longer shows what the bound adds", err)
	}
	result, made, took := run(doc)
	if took > time.Second {
		t.Errorf("took %v, want under a second", took)
	}
	// One list, and at most three calls a read.
	if most := int64(1 + 3*gqlMaxNestedReads); made > most {
		t.Errorf("made %d Kubernetes API calls, want at most %d", made, most)
	}
	if got, want := limited(result), 100*len(nested)*(gqlMaxAliasesPerField+1)-gqlMaxNestedReads; got != want {
		t.Errorf("%d reads refused with %s, want %d", got, errNestedReads.Code, want)
	}

	// Every nested field on one service, as the dashboard reads them, and on
	// a page of services up to the bound, still read.
	for _, doc := range []string{
		"{ services(limit: 1) { id " + strings.Join(nested, " ") + " } }",
		fmt.Sprintf("{ services(limit: %d) { id outboundIps { ips } } }", gqlMaxNestedReads),
	} {
		if result, _, _ := run(doc); limited(result) > 0 {
			t.Errorf("%s: %d reads refused", doc, limited(result))
		}
	}
}

// settingsStore is fakeWSStore plus the inbound IP rules and the Overlapping
// Deploy Policy, counting the store reads of either.
type settingsStore struct {
	*fakeWSStore
	reads atomic.Int64
}

func (s *settingsStore) WorkspaceIPAllowList(ctx context.Context, id string) ([]core.IPAllowListEntry, bool, error) {
	lists, err := s.WorkspaceIPAllowLists(ctx, []string{id})
	entries, ok := lists[id]
	return entries, ok, err
}

func (s *settingsStore) WorkspaceIPAllowLists(context.Context, []string) (map[string][]core.IPAllowListEntry, error) {
	s.reads.Add(1)
	return map[string][]core.IPAllowListEntry{}, nil
}

func (s *settingsStore) SetWorkspaceIPAllowList(context.Context, string, []core.IPAllowListEntry) error {
	return nil
}

func (s *settingsStore) OverlappingDeployPolicies(_ context.Context, ids []string) (map[string]string, error) {
	s.reads.Add(1)
	out := map[string]string{}
	for _, id := range ids {
		out[id] = store.OverlapWait
	}
	return out, nil
}

func (s *settingsStore) SetOverlappingDeployPolicy(context.Context, string, string) error { return nil }

// TestGraphQLWorkspaceSettingsAreReadOncePerList: Workspace's ipAllowList and
// overlappingDeployPolicy each read the store, and under workspaces they
// read it once a workspace and again for each alias. A document the cost gate
// admits, 11 copies of the list with each field aliased 10 times, made 42
// reads a membership: 42,000 store queries and 4 s against a real Postgres for
// a caller in 1,000 workspaces, which an org Owner creates at no fee. Each
// list now reads each setting once for all its rows.
func TestGraphQLWorkspaceSettingsAreReadOncePerList(t *testing.T) {
	const memberships = 100
	st := &settingsStore{fakeWSStore: newFakeWSStore()}
	for i := range memberships {
		mustCreate(t, st.fakeWSStore, fmt.Sprintf("ws%03d", i), "pro", "client-1")
	}
	srv := NewServer(serverBase(t, st), Deps{WorkspaceStore: st})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema

	lists := gqlMaxAliasesPerField + 1
	var doc strings.Builder
	doc.WriteString("{ ")
	for l := range lists {
		if l > 0 {
			fmt.Fprintf(&doc, "w%d: ", l)
		}
		doc.WriteString("workspaces { id ipAllowList { cidrBlock } overlappingDeployPolicy ")
		if l == 0 {
			for i := 1; i <= gqlMaxAliasesPerField; i++ {
				fmt.Fprintf(&doc, "a%d: ipAllowList { cidrBlock } p%d: overlappingDeployPolicy ", i, i)
			}
		}
		doc.WriteString("} ")
	}
	doc.WriteString("}")
	if err := validateGraphQLComplexity(doc.String()); err != nil {
		t.Fatalf("the cost gate refuses the document (%v); it no longer shows what the bound adds", err)
	}

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1"})
	var result *graphql.Result
	finishesWithin(t, "workspaces settings", func() {
		_, _, _, result = srv.executeGraphQL(ctx, graphqlRequestBody{Query: doc.String()}, &gqlBudget{})
	})
	if len(result.Errors) > 0 {
		t.Fatalf("errors: %v", result.Errors)
	}
	if got, want := st.reads.Load(), int64(2*lists); got != want {
		t.Errorf("%d store reads for %d workspaces, want %d: one a setting a list", got, memberships, want)
	}
	// Every row still answers every alias.
	rows, _ := result.Data.(map[string]any)["workspaces"].([]any)
	if len(rows) != memberships {
		t.Fatalf("%d workspaces listed, want %d", len(rows), memberships)
	}
	for _, r := range rows {
		row := r.(map[string]any)
		for i := 1; i <= gqlMaxAliasesPerField; i++ {
			if rules, _ := row[fmt.Sprintf("a%d", i)].([]any); len(rules) == 0 || row[fmt.Sprintf("p%d", i)] != store.OverlapWait {
				t.Fatalf("row %v: a%d = %v, p%d = %v; want the default rules and wait", row["id"], i, row[fmt.Sprintf("a%d", i)], i, row[fmt.Sprintf("p%d", i)])
			}
		}
	}

	// The polled list selects neither and reads neither.
	st.reads.Store(0)
	if _, _, _, result = srv.executeGraphQL(ctx, graphqlRequestBody{Query: "{ workspaces { id name plan } }"}, &gqlBudget{}); len(result.Errors) > 0 || st.reads.Load() != 0 {
		t.Errorf("the plain list: errors %v, %d settings reads; want none", result.Errors, st.reads.Load())
	}
}
