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
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// public_routing_condition_test.go covers w7/m79/t003.
//
// w7/m78 established that production runs with BEX_BASE_DOMAIN unset (a
// deliberate security decision — onbex.co is a registrable domain), so a web
// service with no custom domain has zero effective hosts, gets no Ingress, and
// reports url: null. All of that is correct. The defect was that nothing said
// so, in a form the user could act on.
//
// The hard part of this condition is not firing it — it is NOT firing it for the
// states someone chose on purpose. A condition that shows up on every worker and
// every deliberately-private service teaches people to ignore it, which is
// strictly worse than the silence being replaced. Hence a negative case per
// exclusion.

func routingApp(t string, expose bool, policy string) *appv1alpha1.App {
	return &appv1alpha1.App{
		Spec: appv1alpha1.AppSpec{Type: t, Expose: expose, SubdomainPolicy: policy},
	}
}

func routingCondition(app *appv1alpha1.App) *metav1.Condition {
	return meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionPublicRouting)
}

func TestPublicRoutingReportsAnUnroutableWebService(t *testing.T) {
	r := &AppReconciler{}
	app := routingApp(appv1alpha1.TypeWebService, true, "")

	r.setPublicRoutingCondition(app, nil)

	c := routingCondition(app)
	if c == nil {
		t.Fatal("no PublicRouting condition: an exposed service with no host is exactly the silent state m79 closes")
	}
	if c.Status != metav1.ConditionFalse || c.Reason != "NoPublicHost" {
		t.Fatalf("condition = %s/%s, want False/NoPublicHost", c.Status, c.Reason)
	}
	// A reason the user cannot act on is barely better than silence.
	if !strings.Contains(c.Message, "custom domain") {
		t.Errorf("message names no action to take: %s", c.Message)
	}
}

func TestPublicRoutingClearsOnceAHostExists(t *testing.T) {
	r := &AppReconciler{}
	app := routingApp(appv1alpha1.TypeWebService, true, "")

	r.setPublicRoutingCondition(app, nil)
	if c := routingCondition(app); c == nil || c.Status != metav1.ConditionFalse {
		t.Fatal("precondition: the unroutable condition should be set")
	}

	// Attaching a custom domain is the documented remedy; the condition must
	// follow reality rather than latch.
	r.setPublicRoutingCondition(app, []string{"forum.example.com"})

	c := routingCondition(app)
	if c == nil || c.Status != metav1.ConditionTrue || c.Reason != "Routed" {
		t.Fatalf("condition = %+v, want True/Routed once a host exists", c)
	}
	if !strings.Contains(c.Message, "forum.example.com") {
		t.Errorf("routed message does not name the host: %s", c.Message)
	}
}

// TestPublicRoutingStaysQuietForDeliberateStates is the load-bearing negative.
func TestPublicRoutingStaysQuietForDeliberateStates(t *testing.T) {
	r := &AppReconciler{}
	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
		why  string
	}{
		{
			name: "background worker",
			app:  routingApp(appv1alpha1.TypeBackgroundWorker, true, ""),
			why:  "a worker has no public route by definition",
		},
		{
			name: "cron job",
			app:  routingApp(appv1alpha1.TypeCronJob, true, ""),
			why:  "a cron job has no serving URL",
		},
		{
			name: "private service",
			app:  routingApp(appv1alpha1.TypePrivateService, true, ""),
			why:  "a private service is in-cluster only on purpose",
		},
		{
			name: "not exposed",
			app:  routingApp(appv1alpha1.TypeWebService, false, ""),
			why:  "the service did not ask to be publicly reachable",
		},
		{
			name: "platform subdomain switched off by its owner",
			app:  routingApp(appv1alpha1.TypeWebService, true, appv1alpha1.SubdomainPolicyDisabled),
			why:  "renderSubdomainPolicy: disabled is the owner's explicit choice (w7/m31)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r.setPublicRoutingCondition(tc.app, nil)
			if c := routingCondition(tc.app); c != nil {
				t.Errorf("reported %s/%s — %s, so this must stay quiet", c.Status, c.Reason, tc.why)
			}
		})
	}
}

// TestPublicRoutingConditionIsNotReady pins the separation. Ready is the health
// signal; a service with no public address can be entirely healthy, and marking
// it unhealthy would be wrong in one direction while hiding the routing state in
// the other. Every existing consumer filters on Type == "Ready", so this
// separation is also what keeps the change additive.
func TestPublicRoutingConditionIsNotReady(t *testing.T) {
	r := &AppReconciler{}
	app := routingApp(appv1alpha1.TypeWebService, true, "")

	r.setPublicRoutingCondition(app, nil)

	if meta.FindStatusCondition(app.Status.Conditions, "Ready") != nil {
		t.Error("setPublicRoutingCondition wrote the Ready condition; routing state must not masquerade as health")
	}
	if appv1alpha1.ConditionPublicRouting == "Ready" {
		t.Fatal("the routing condition must not reuse the Ready type")
	}
}
