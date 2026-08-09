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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// public_routing_notice_test.go covers the API half of w7/m79/t003: carrying the
// operator's PublicRouting verdict to the surfaces a tenant actually reads.
//
// A tenant never runs kubectl, so a condition on the CR answers nobody's
// question. The question is "why is my service's url null?", and it is asked of
// the API — so that is where the answer has to appear.

func appWithRoutingCondition(status metav1.ConditionStatus, reason, message string) *appv1alpha1.App {
	return &appv1alpha1.App{
		Status: appv1alpha1.AppStatus{
			Conditions: []metav1.Condition{{
				Type: appv1alpha1.ConditionPublicRouting, Status: status,
				Reason: reason, Message: message,
			}},
		},
	}
}

func TestViewCarriesTheRoutingNotice(t *testing.T) {
	const msg = "this service has no public address: the platform subdomain is not available on this " +
		"installation, and no custom domain is attached. Add a custom domain to serve it publicly."
	v := view(appWithRoutingCondition(metav1.ConditionFalse, "NoPublicHost", msg))

	if v.PublicRoutingNotice != msg {
		t.Errorf("publicRoutingNotice = %q, want the operator's message", v.PublicRoutingNotice)
	}
	if !strings.Contains(v.PublicRoutingNotice, "custom domain") {
		t.Error("the notice reaching the API names no action to take")
	}
}

// TestViewIsQuietWhenRouted pins the negative: a routed service, and a service
// the operator deliberately said nothing about, must both carry no notice. A
// field that is always populated is noise, and noise is what the silence this
// milestone replaces would become.
func TestViewIsQuietWhenRouted(t *testing.T) {
	routed := view(appWithRoutingCondition(metav1.ConditionTrue, "Routed", "serving at forum.example.com"))
	if routed.PublicRoutingNotice != "" {
		t.Errorf("a routed service carries a notice: %q", routed.PublicRoutingNotice)
	}

	// No condition at all — a worker, a cron job, a private service, or an owner
	// who switched their own platform subdomain off.
	none := view(&appv1alpha1.App{})
	if none.PublicRoutingNotice != "" {
		t.Errorf("a service with no routing condition carries a notice: %q", none.PublicRoutingNotice)
	}
}

// TestRoutingNoticeReadsTheOperatorsVerdictNotItsOwn pins the layering decision.
// bex-api could recompute "has a public host" from EffectiveHosts, but it holds
// its own BEX_BASE_DOMAIN, which can differ from the operator's (.pm/w7/026.md).
// Only the operator knows what actually got an Ingress, so the API reports its
// verdict rather than a second opinion.
func TestRoutingNoticeReadsTheOperatorsVerdictNotItsOwn(t *testing.T) {
	// A service that looks routable by spec alone — exposed, with a subdomain —
	// but which the operator has reported as unroutable.
	app := appWithRoutingCondition(metav1.ConditionFalse, "NoPublicHost", "no public address")
	app.Spec.Expose = true
	app.Spec.Subdomain = "forum"
	app.Spec.Type = appv1alpha1.TypeWebService

	if got := view(app).PublicRoutingNotice; got == "" {
		t.Error("the API second-guessed the operator and dropped the notice for a spec that merely looks routable")
	}
}
