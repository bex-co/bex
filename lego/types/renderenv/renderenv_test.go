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

package renderenv

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRenderRepoSlugKeepsOwnerAndRepoInAnyURLForm(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/Acme/Web.git":    "Acme/Web",
		"https://github.com/acme/web/":       "acme/web",
		"git@github.com:acme/web.git":        "acme/web",
		"ssh://git@gitlab.com/group/sub/app": "sub/app",
		"https://github.com/acme":            "",
		"":                                   "",
	} {
		if got := RepoSlug(in); got != want {
			t.Errorf("RepoSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDiscoveryServiceNameFitsADNSLabel(t *testing.T) {
	long := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("a", 60)}}
	if n := DiscoveryServiceName(long); len(n) > 63 || !strings.HasSuffix(n, "-discovery") {
		t.Fatalf("discoveryServiceName = %q (%d)", n, len(n))
	}
}

func TestPlanCPUCountRoundsPlanCPUsUp(t *testing.T) {
	for tier, want := range map[string]int{
		"free": 1, "starter": 1, "standard": 1, "pro": 2, "pro-plus": 4, "pro-ultra": 8, "": 1, "no-such-plan": 1,
	} {
		if got := PlanCPUCount(tier); got != want {
			t.Errorf("PlanCPUCount(%q) = %d, want %d", tier, got, want)
		}
	}
}
