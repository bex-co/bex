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

package deploys

import (
	"context"
	"testing"
)

// kick_test.go is w8/270. The deploy row's started time and status are stamped
// by the store reconciler, which runs every 30 seconds unless an API write
// kicks it. No deploy verb did, so a trigger or a cancel was followed at the
// next resync.
func TestDeployVerbsKickTheStoreReconciler(t *testing.T) {
	ctx := context.Background()
	ds := newFakeStore()
	app := sampleApp("web", "srv-1")
	svc, _ := newService(ds, app)
	kicks := 0
	svc.Kick = func() { kicks++ }

	d, err := svc.Trigger(ctx, "web", TriggerParams{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if kicks != 1 {
		t.Fatalf("a trigger kicked the reconciler %d times, want once", kicks)
	}
	if _, err := svc.Cancel(ctx, "web", d.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if kicks != 2 {
		t.Fatalf("a cancel kicked the reconciler %d times in all, want twice", kicks)
	}

	// A verb that is refused before it touches anything kicks nothing, and a
	// service wired without a reconciler still works.
	if _, err := svc.Trigger(ctx, "no-such-service", TriggerParams{}); err == nil || kicks != 2 {
		t.Fatalf("a refused trigger: err %v, kicks %d; want an error and no kick", err, kicks)
	}
	svc.Kick = nil
	if _, err := svc.Trigger(ctx, "web", TriggerParams{}); err != nil {
		t.Fatalf("Trigger without a reconciler: %v", err)
	}
}
