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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func outboundIPTestApp() *appv1alpha1.App { return instanceTestApp() }

func outboundIPNode(name, pool string, addresses ...corev1.NodeAddress) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{"bex.co/pool": pool}},
		Status:     corev1.NodeStatus{Addresses: addresses},
	}
}

func externalIP(ip string) corev1.NodeAddress {
	return corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: ip}
}

func internalIP(ip string) corev1.NodeAddress {
	return corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: ip}
}

func TestOutboundIPsReturnsTenantPoolExternalIPs(t *testing.T) {
	app := outboundIPTestApp()
	tenantA := outboundIPNode("tenant-a", "tenant", internalIP("10.0.0.1"), externalIP("203.0.113.10"))
	tenantB := outboundIPNode("tenant-b", "tenant", externalIP("203.0.113.11"), externalIP("203.0.113.10")) // duplicate IP deduped
	tenantNoExternal := outboundIPNode("tenant-c", "tenant", internalIP("10.0.0.3"))
	platform := outboundIPNode("platform-a", "platform", externalIP("203.0.113.99")) // wrong pool: excluded
	sandbox := outboundIPNode("sandbox-a", "sandbox", externalIP("203.0.113.98"))    // wrong pool: excluded

	svc := instanceService(app, tenantA, tenantB, tenantNoExternal, platform, sandbox)
	got, err := svc.OutboundIPs(context.Background(), testServiceID)
	if err != nil {
		t.Fatalf("OutboundIPs: %v", err)
	}
	if got.Type != "shared" || got.DedicatedIPID != "" {
		t.Fatalf("OutboundIPs = %+v, want type=shared with no dedicatedIpId", got)
	}
	want := []string{"203.0.113.10", "203.0.113.11"}
	if len(got.IPs) != len(want) {
		t.Fatalf("ips = %v, want %v", got.IPs, want)
	}
	for i := range want {
		if got.IPs[i] != want[i] {
			t.Fatalf("ips = %v, want sorted/deduped %v", got.IPs, want)
		}
	}
}

func TestOutboundIPsEmptyWhenPoolHasNoExternalIPs(t *testing.T) {
	app := outboundIPTestApp()
	// The local CAPD mock's truthful answer: pool nodes without ExternalIPs.
	node := outboundIPNode("capd-worker", "tenant", internalIP("172.18.0.2"))
	svc := instanceService(app, node)
	got, err := svc.OutboundIPs(context.Background(), testServiceID)
	if err != nil {
		t.Fatalf("OutboundIPs: %v", err)
	}
	if got.Type != "shared" || got.IPs == nil || len(got.IPs) != 0 {
		t.Fatalf("OutboundIPs = %+v, want type=shared with an allocated empty ips", got)
	}
}

type nodeListTrapClient struct {
	client.Client
	nodeListCalled bool
}

func (c *nodeListTrapClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*corev1.NodeList); ok {
		c.nodeListCalled = true
		return errors.New("Node list must not run before authorization")
	}
	return c.Client.List(ctx, list, opts...)
}

func TestOutboundIPsAuthorizesBeforeObservingNodes(t *testing.T) {
	app := outboundIPTestApp()
	trap := &nodeListTrapClient{Client: fakeClient(app)}
	svc := &Service{Base: &core.Base{
		Client: trap, Namespace: "default", Authz: &fakeChecker{allow: false},
	}}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "intruder", Method: "oauth2"})
	if _, err := svc.OutboundIPs(ctx, testServiceID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("OutboundIPs error = %v, want ErrForbidden", err)
	}
	if trap.nodeListCalled {
		t.Fatal("OutboundIPs observed Nodes before App authorization")
	}
}

func TestGraphQLServiceOutboundIps(t *testing.T) {
	app := outboundIPTestApp()
	node := outboundIPNode("tenant-a", "tenant", externalIP("203.0.113.10"))
	svc := instanceService(app, node)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	query := `{ service(id: "` + testServiceID + `") { outboundIps { type dedicatedIpId ips } } }`

	// The composition root injects the reader seam (core.WithOutboundIPs); the
	// shared Service type resolves the field through it.
	ctx := core.WithOutboundIPs(context.Background(), svc)
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: query})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["service"].(map[string]any)["outboundIps"].(map[string]any)
	if got["type"] != "shared" || got["dedicatedIpId"] != nil {
		t.Fatalf("outboundIps = %#v, want type=shared, dedicatedIpId null", got)
	}
	ips, ok := got["ips"].([]any)
	if !ok || len(ips) != 1 || ips[0] != "203.0.113.10" {
		t.Fatalf("ips = %#v, want [203.0.113.10]", got["ips"])
	}

	// Unwired (no reader injected — a server without Apps) fails closed.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
	if len(res.Errors) == 0 {
		t.Fatalf("unwired outboundIps = %#v, want an error", res.Data)
	}
}

func TestRESTServiceOutboundIPsExactWireShape(t *testing.T) {
	app := outboundIPTestApp()
	node := outboundIPNode("tenant-a", "tenant", externalIP("203.0.113.10"))
	svc := instanceService(app, node)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	path := "/v1/services/" + testServiceID + "/outbound-ips"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if len(raw) != 2 || raw["type"] != "shared" {
		t.Fatalf("wire shape = %#v, want exactly {type, ips} with type=shared (dedicatedIpId omitted)", raw)
	}
	ips, ok := raw["ips"].([]any)
	if !ok || len(ips) != 1 || ips[0] != "203.0.113.10" {
		t.Fatalf("ips = %#v, want [203.0.113.10]", raw["ips"])
	}
}

func TestRESTServiceOutboundIPsEmptyAndMissing(t *testing.T) {
	t.Run("empty ips is an array not null", func(t *testing.T) {
		mux := http.NewServeMux()
		instanceService(outboundIPTestApp()).RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/"+testServiceID+"/outbound-ips", nil))
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"type":"shared","ips":[]}` {
			t.Fatalf("empty outbound-ips = %d %q, want 200 {\"type\":\"shared\",\"ips\":[]}", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing uses shared error envelope", func(t *testing.T) {
		mux := http.NewServeMux()
		instanceService().RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/srv-missing/outbound-ips", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("missing = %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["id"] != "not_found" || body["message"] == nil || body["error"] == nil {
			t.Fatalf("missing error envelope = %#v, err=%v", body, err)
		}
	})

	t.Run("foreign workspace is forbidden before Node list", func(t *testing.T) {
		app := outboundIPTestApp()
		app.Labels[core.LabelTenant] = "tea-b"
		trap := &nodeListTrapClient{Client: fakeClient(app)}
		svc := &Service{Base: &core.Base{
			Client: trap, Namespace: "default", Workspace: fakeWorkspace{"alice": "tea-a"},
		}}
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		req := httptest.NewRequest(http.MethodGet, "/v1/services/"+testServiceID+"/outbound-ips", nil)
		req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "alice", Method: "session"}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || trap.nodeListCalled {
			t.Fatalf("foreign workspace = %d nodeList=%v body=%s", rec.Code, trap.nodeListCalled, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "203.0.113") {
			t.Fatalf("foreign response leaked Node data: %s", rec.Body.String())
		}
	})
}
