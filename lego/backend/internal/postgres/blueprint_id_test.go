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

package postgres

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Blueprint-managed database reports the public exs- id; its label keeps
// the durable blp- key (w2/m269).
func TestPostgresViewBlueprintIDIsPublic(t *testing.T) {
	durable := ids.New(ids.Blueprint)
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "db", Labels: map[string]string{core.LabelBlueprint: durable}}}
	if got := pgView(db).BlueprintID; got != ids.BlueprintPublicID(durable) {
		t.Fatalf("PostgresView.BlueprintID = %q, want %q", got, ids.BlueprintPublicID(durable))
	}
	if db.Labels[core.LabelBlueprint] != durable {
		t.Fatal("projecting the view rewrote the durable label")
	}
}
