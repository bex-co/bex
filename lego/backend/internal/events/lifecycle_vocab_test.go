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
