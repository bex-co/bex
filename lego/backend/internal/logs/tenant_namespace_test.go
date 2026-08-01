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

package logs

import (
	"context"
	"io"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// tenant_namespace_test.go is the regression guard for the ADR043 per-tenant
// namespace LOG read-path bug (w3/m36): once BEX_TENANT_NAMESPACES was enabled
// the projector moved every App CR — and so its pods and Loki streams — into the
// workspace's own `<ws>` namespace (== the workspace id), but the logs feature
// kept querying the shared BEX_API_NAMESPACE ("default"). The LogQL selector
// became `{namespace="default", app=…}`, which matches nothing, so the dashboard
// Logs tab was always empty for every tenant service. The metrics sibling was
// already fixed (internal/metrics uses app.Namespace); these tests pin the logs
// fix: every log read path resolves into the App's own namespace.

const logTestWS = "tea-d98210cbbpdc73dcrkvg"

// tenantNSApp builds an App CR exactly as the projector projects it under
// per-tenant namespaces: in the `<ws>` namespace, object-named CRName(ws, svc),
// carrying the tenant + public-name + app-id labels the read path resolves against.
func logTenantNSApp(svc, ws, appID string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      core.CRName(ws, svc),
			Namespace: ws,
			Labels: map[string]string{
				core.LabelTenant:      ws,
				core.LabelServiceName: svc,
				core.LabelAppID:       appID,
			},
		},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning},
	}
}

// capturingHistory records the namespace of the most recent history read — the
// exact value that becomes the LogQL selector's `namespace=` label.
func capturingHistory(ns *string) LogHistorySource {
	return func(_ context.Context, namespace string, _ LogQuery) ([]LogEntry, error) {
		*ns = namespace
		return nil, nil
	}
}

func capturingLabelValues(ns *string) LogLabelValuesSource {
	return func(_ context.Context, namespace, _ string, _ LogQuery) ([]string, error) {
		*ns = namespace
		return nil, nil
	}
}

// capturingPodLogs records the namespace each pod-log read targets.
func capturingPodLogs(ns *string) PodLogSource {
	return func(_ context.Context, namespace, _, _ string, _ int64) (io.ReadCloser, error) {
		*ns = namespace
		return io.NopCloser(strings.NewReader("")), nil
	}
}

// TestQueryLogs_TenantNamespace_History is the direct reproduction: the durable
// (Loki) read for a tenant service must query the App's `<ws>` namespace, never
// the shared BEX_API_NAMESPACE — otherwise the stream selector matches nothing.
func TestQueryLogs_TenantNamespace_History(t *testing.T) {
	t.Run("tenant namespaces on -> App's <ws> namespace", func(t *testing.T) {
		var gotNS string
		cl := fakeClientWith(logTenantNSApp("web", logTestWS, "srv-1"))
		svc := &Service{
			Base:    &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true},
			History: capturingHistory(&gotNS),
		}
		if _, err := svc.QueryLogs(context.Background(), LogQuery{App: "srv-1"}); err != nil {
			t.Fatalf("QueryLogs: %v", err)
		}
		if gotNS != logTestWS {
			t.Fatalf("history queried namespace %q, want the App's <ws> namespace %q", gotNS, logTestWS)
		}
	})

	t.Run("shared namespace off -> BEX_API_NAMESPACE (byte-identical)", func(t *testing.T) {
		var gotNS string
		app := sampleApp("web") // in "default"
		app.Labels = map[string]string{core.LabelAppID: "srv-1"}
		cl := fakeClientWith(app)
		svc := &Service{
			Base:    &core.Base{Client: cl, Namespace: "default"},
			History: capturingHistory(&gotNS),
		}
		if _, err := svc.QueryLogs(context.Background(), LogQuery{App: "srv-1"}); err != nil {
			t.Fatalf("QueryLogs: %v", err)
		}
		if gotNS != "default" {
			t.Fatalf("shared-namespace history queried %q, want default", gotNS)
		}
	})
}

// TestLogs_TenantNamespace_History pins the MCP convenience read (Logs) on the
// same resolution as QueryLogs.
func TestLogs_TenantNamespace_History(t *testing.T) {
	var gotNS string
	cl := fakeClientWith(logTenantNSApp("web", logTestWS, "srv-1"))
	svc := &Service{
		Base:    &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true},
		History: capturingHistory(&gotNS),
	}
	if _, err := svc.Logs(context.Background(), "srv-1", 0); err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if gotNS != logTestWS {
		t.Fatalf("Logs history queried namespace %q, want %q", gotNS, logTestWS)
	}
}

// TestLogLabelValues_TenantNamespace pins the filter-dropdown discovery read.
func TestLogLabelValues_TenantNamespace(t *testing.T) {
	var gotNS string
	cl := fakeClientWith(logTenantNSApp("web", logTestWS, "srv-1"))
	svc := &Service{
		Base:        &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true},
		LabelValues: capturingLabelValues(&gotNS),
	}
	if _, err := svc.LogLabelValues(context.Background(), LabelLevel, LogQuery{App: "srv-1"}); err != nil {
		t.Fatalf("LogLabelValues: %v", err)
	}
	if gotNS != logTestWS {
		t.Fatalf("label-values queried namespace %q, want %q", gotNS, logTestWS)
	}
}

// TestQueryLogs_TenantNamespace_PodLogFallback proves the live pod-log fallback
// (no store wired) reads the pod's container from the App's `<ws>` namespace —
// the same namespace appPodNames/AppPods selected the pod in.
func TestQueryLogs_TenantNamespace_PodLogFallback(t *testing.T) {
	crName := core.CRName(logTestWS, "web")
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      crName + "-abc123",
		Namespace: logTestWS,
		Labels:    map[string]string{core.PodLabelApp: crName},
	}}
	var gotNS string
	cl := fakeClientWith(logTenantNSApp("web", logTestWS, "srv-1"), pod)
	svc := &Service{
		Base:    &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true},
		PodLogs: capturingPodLogs(&gotNS), // no History => live pod-log fallback
	}
	if _, err := svc.QueryLogs(context.Background(), LogQuery{App: "srv-1"}); err != nil {
		t.Fatalf("QueryLogs (pod fallback): %v", err)
	}
	if gotNS != logTestWS {
		t.Fatalf("pod-log read targeted namespace %q, want the App's <ws> namespace %q", gotNS, logTestWS)
	}
}
