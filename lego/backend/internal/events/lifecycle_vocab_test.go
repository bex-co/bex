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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestPushDownRoutesEveryFactType keeps the fact vocabulary consistent: every
// type in allFactTypes must push down to a single-fact-type store filter (never
// an audit-verb or deploy-phase filter), so a type filter selects exactly its
// rows. A type that fell out of the switch would silently match nothing.
func TestPushDownRoutesEveryFactType(t *testing.T) {
	for _, ft := range allFactTypes {
		verbs, phases, factTypes, ad := pushDown(ft)
		if len(verbs) != 0 || len(phases) != 0 || ad != store.AutoDeployFilterNone {
			t.Errorf("%s: pushDown returned non-fact filters verbs=%v phases=%v ad=%v", ft, verbs, phases, ad)
		}
		if len(factTypes) != 1 || factTypes[0] != ft {
			t.Errorf("%s: pushDown factTypes = %v, want [%s]", ft, factTypes, ft)
		}
	}
}

// TestCronRunEventsComeFromObservedFactsNotIntentVerbs is the regression for
// w4/m118, and the feed's copy of the invariant webhooks already hold (see
// webhooks' TestCronWebhookEventsComeFromObservedFactsNotIntentVerbs).
//
// An every-minute cron whose first scheduled run failed after 10m 54s of
// crash-loop backoff showed nothing in Activity — only the two deploy events —
// while Recent Runs said Failed with a duration, webhooks fired, and push said
// "Cron run failed". The feed mapped the three INTENT verbs (somebody asked for
// a run, or asked for one to stop) and omitted both observed fact types from
// allFactTypes, so it could only ever show manual actions.
//
// Both halves have to hold together. Re-adding the verbs would show every
// manual run twice; dropping the facts brings back the invisible schedule.
func TestCronRunEventsComeFromObservedFactsNotIntentVerbs(t *testing.T) {
	for _, verb := range []string{"apps.TriggerCronRun", "apps.CancelCronRun", "apps.CancelCurrentCronRun"} {
		if eventType, ok := eventTypes[verb]; ok {
			t.Errorf("intent verb %q maps to %q — a manual run would appear twice", verb, eventType)
		}
		if slices.Contains(allVerbs, verb) {
			t.Errorf("intent verb %q is still in the feed's audit-verb query", verb)
		}
	}
	for _, ft := range []string{TypeCronJobRunStarted, TypeCronJobRunEnded} {
		if !slices.Contains(allFactTypes, ft) {
			t.Errorf("%s missing from allFactTypes — a scheduled run stays invisible in an unfiltered feed", ft)
		}
		verbs, phases, factTypes, ad := pushDown(ft)
		if len(factTypes) != 1 || factTypes[0] != ft {
			t.Errorf("%s: filtering by it must ask the store for exactly its fact rows, got %v", ft, factTypes)
		}
		if len(verbs) != 0 || len(phases) != 0 || ad != store.AutoDeployFilterNone {
			t.Errorf("%s: filtering by it must not fall back to verbs/phases: %v %v %v", ft, verbs, phases, ad)
		}
	}
}

// TestLifecycleTypesInVocabulary pins the w7/m66 additions into the unfiltered
// feed's fact-type set — omission would make them invisible to a "show all" read.
func TestLifecycleTypesInVocabulary(t *testing.T) {
	for _, ft := range []string{
		TypeBuildStarted, TypeBuildEnded,
		TypePreDeployStarted, TypePreDeployEnded,
		TypeJobRunEnded, TypeBranchDeleted,
	} {
		if !slices.Contains(allFactTypes, ft) {
			t.Errorf("%s missing from allFactTypes — it would never appear in an unfiltered feed", ft)
		}
	}
}

// TestObservedDatastoreTypesStayOutOfTheServiceFeed pins the w3/m82 t004
// boundary. The datastore fact types are advertised on outbound webhooks and
// retrievable by evt-… id, but they live in datastore_event_facts and belong to
// a dpg-/red- resource with no apps row — GET /services/{id}/events can never
// join them. Listing them in allFactTypes would push a filter down onto
// service_event_facts that matches nothing, which reads as "this event does not
// exist" rather than "this feed is not its home".
func TestObservedDatastoreTypesStayOutOfTheServiceFeed(t *testing.T) {
	for _, ft := range []string{
		TypePostgresUnavailable, TypePostgresAvailable,
		TypeKeyValueUnhealthy, TypeKeyValueAvailable,
		TypePostgresBackupCompleted, TypePostgresBackupFailed,
		TypePostgresRestoreSucceeded, TypePostgresRestoreFailed,
		TypePostgresUpgradeStarted, TypePostgresUpgradeSucceeded, TypePostgresUpgradeFailed,
	} {
		if slices.Contains(allFactTypes, ft) {
			t.Errorf("%s is in allFactTypes — the service feed would filter service_event_facts by a datastore fact type and match nothing", ft)
		}
	}
}
