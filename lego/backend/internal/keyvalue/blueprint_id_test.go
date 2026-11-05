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

package keyvalue

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Blueprint-managed key value reports the public exs- id on the view and
// its Render-shaped REST body; its label keeps the durable blp- key (w2/m269).
func TestKeyValueViewBlueprintIDIsPublic(t *testing.T) {
	durable := ids.New(ids.Blueprint)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "cache", Labels: map[string]string{core.LabelBlueprint: durable}}}
	if got := kvView(kv).BlueprintID; got != ids.BlueprintPublicID(durable) {
		t.Fatalf("KeyValueView.BlueprintID = %q, want %q", got, ids.BlueprintPublicID(durable))
	}
	if kv.Labels[core.LabelBlueprint] != durable {
		t.Fatal("projecting the view rewrote the durable label")
	}
}
