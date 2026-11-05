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

package notifications

import (
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The failed-sync email links the Blueprint page by the public exs- id the
// dashboard routes by, while the stream keeps the durable blp- key (w2/m269).
func TestBlueprintAlertEmailLinksThePublicID(t *testing.T) {
	durable := ids.New(ids.Blueprint)
	w := &AlertEmailWorker{DashboardBaseURL: "https://dashboard.example"}
	got := w.dashboardURL(store.AlertEmailStream{Event: store.AlertEmailBlueprintSyncFailed, ResourceID: durable})
	if want := "https://dashboard.example/blueprints/" + ids.BlueprintPublicID(durable); got != want {
		t.Fatalf("dashboardURL = %q, want %q", got, want)
	}
}
