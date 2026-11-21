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
