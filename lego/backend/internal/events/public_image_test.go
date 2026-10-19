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

package events

import (
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m236: deploy and fact event details carry the public image form, never
// the cluster-internal build registry host, on every adapter reading Details.
func TestEventViewProjectsPublicImageRef(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	internal := "zot.bex-registry.svc:5000/ws/ws-svc:gen-2@" + digest
	want := "ws/ws-svc:gen-2@" + digest
	for _, row := range []store.ServiceEventRow{
		{Key: "dep-1:started", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-1", Trigger: store.TriggerAPI, Image: internal},
		{Key: "dep-1:ended", Source: store.EventSourceFact, DeployID: "dep-1", Image: internal},
	} {
		if got := view(row, "srv-1").Details.Image; got != want {
			t.Errorf("source %v details image = %q; want %q", row.Source, got, want)
		}
	}
	external := "ghcr.io/acme/web:v1"
	row := store.ServiceEventRow{Key: "dep-2:started", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-2", Trigger: store.TriggerAPI, Image: external}
	if got := view(row, "srv-1").Details.Image; got != external {
		t.Errorf("details image = %q; want byte-identical %q", got, external)
	}
}
