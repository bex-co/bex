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

package agentsessions

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// flagMarks is the ModelKeyFlagMarker fake: flagged workspaces, the gone
// ones, and one whose write fails.
type flagMarks struct {
	flagged map[string]bool
	gone    map[string]bool
	failing string
}

func (f *flagMarks) MarkAgentModelKeySet(_ context.Context, ws string) (bool, error) {
	switch {
	case f.gone[ws]:
		return false, store.ErrNotFound
	case ws == f.failing:
		return false, errors.New("db down")
	case f.flagged[ws]:
		return false, nil
	}
	f.flagged[ws] = true
	return true, nil
}

// The startup backfill flags every workspace whose model key sits in
// OpenBao without a flag (a hand-provisioned key), leaves flagged ones,
// workspaces without a key and deleted workspaces alone, and counts failures.
func TestBackfillModelKeyFlags(t *testing.T) {
	key := func(v string) map[string]string { return map[string]string{agentsession.ModelKeyField: v} }
	keys := &fakeModelKeys{data: map[string]map[string]string{
		agentsession.ModelKeySecretPath("tea-hand"):    key("sk-ant-api03-HAND"),
		agentsession.ModelKeySecretPath("tea-flagged"): key("sk-ant-api03-FLAGGED"),
		agentsession.ModelKeySecretPath("tea-gone"):    key("sk-ant-api03-GONE"),
		agentsession.ModelKeySecretPath("tea-broken"):  key("sk-ant-api03-BROKEN"),
		agentsession.ModelKeySecretPath("tea-empty"):   key(""),
		"agent-sessions/tea-other/github":              {"token": "x"},
	}}
	marks := &flagMarks{flagged: map[string]bool{"tea-flagged": true}, gone: map[string]bool{"tea-gone": true}, failing: "tea-broken"}
	got, err := BackfillModelKeyFlags(context.Background(), keys, marks)
	if err != nil {
		t.Fatal(err)
	}
	if want := (ModelKeyBackfill{Workspaces: 6, Keys: 4, Marked: 1, Failed: 1}); got != want {
		t.Fatalf("backfill = %+v, want %+v", got, want)
	}
	if want := map[string]bool{"tea-flagged": true, "tea-hand": true}; !reflect.DeepEqual(marks.flagged, want) {
		t.Fatalf("flagged = %v, want %v", marks.flagged, want)
	}

	// A second pass (the next start) finds nothing new.
	marks.failing = ""
	if got, err := BackfillModelKeyFlags(context.Background(), keys, marks); err != nil || got.Marked != 1 || got.Failed != 0 {
		t.Fatalf("second pass = %+v err=%v, want only tea-broken newly flagged", got, err)
	}
	keys.err = errors.New("openbao down")
	if _, err := BackfillModelKeyFlags(context.Background(), keys, marks); err == nil {
		t.Fatal("an unlistable OpenBao reported success")
	}
}
