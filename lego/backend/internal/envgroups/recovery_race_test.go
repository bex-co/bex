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

package envgroups

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type pausedGroupKV struct {
	*fakeStore
	armed   atomic.Bool
	verb    string
	suffix  string
	entered chan struct{}
	resume  chan struct{}
}

func (f *pausedGroupKV) pause(verb, path string) {
	if verb == f.verb && strings.HasSuffix(path, f.suffix) && f.armed.CompareAndSwap(true, false) {
		close(f.entered)
		<-f.resume
	}
}

func (f *pausedGroupKV) Get(ctx context.Context, path string) (map[string]string, error) {
	values, err := f.fakeStore.Get(ctx, path)
	f.pause("Get", path)
	return values, err
}

func (f *pausedGroupKV) Put(ctx context.Context, path string, values map[string]string) error {
	err := f.fakeStore.Put(ctx, path, values)
	f.pause("Put", path)
	return err
}

func TestTwoReplicasCannotRecoverOverEachOthersContentEffects(t *testing.T) {
	for _, paused := range []string{"recovery", "original writer past its lease"} {
		t.Run(paused, func(t *testing.T) {
			ctx := testCtx()
			kv := &pausedGroupKV{fakeStore: newFakeStore(), entered: make(chan struct{}), resume: make(chan struct{})}
			writer := newService(kv)
			g, err := writer.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			independent, err := writer.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "independent"})
			if err != nil {
				t.Fatal(err)
			}
			replica := newService(kv)
			replica.Client = writer.Client
			replica.Base.Clock = func() time.Time { return writer.nowTime().Add(time.Hour) }
			if paused == "recovery" {
				strand(t, writer, g.ID, opKindPatch, opPhaseEnvWritten, map[string]string{"TOKEN": "before"}, map[string]string{"TOKEN": "half"})
				if err := writer.writeSlot(ctx, g.OwnerID, g.ID, envSlot, map[string]string{"TOKEN": "half"}); err != nil {
					t.Fatal(err)
				}
				writer.Base.Clock = func() time.Time { return time.Unix(1_000_000, 0).UTC().Add(time.Hour) }
				kv.verb, kv.suffix = "Get", "/old-env"
			} else {
				kv.verb, kv.suffix = "Put", "/env"
			}
			stale, _ := writer.readGroup(ctx, g.ID)
			kv.armed.Store(true)
			first := make(chan error, 1)
			go func() {
				if paused == "recovery" {
					_, err := writer.GetEnvGroupVar(ctx, g.ID, "TOKEN")
					first <- err
				} else {
					_, err := writer.SetEnvGroupVar(ctx, g.ID, "TOKEN", "writer")
					first <- err
				}
			}()
			<-kv.entered
			// Both paths would observe an expired operation, but its original live
			// instance still owns the side-effect lock. They must fail before KV or
			// projection changes, instead of discovering the conflict at release.
			if _, err := replica.GetEnvGroupVar(ctx, g.ID, "TOKEN"); !errors.Is(err, core.ErrConflict) {
				close(kv.resume)
				<-first
				t.Fatalf("second replica's reveal = %v, want conflict", err)
			}
			if err := replica.DeleteEnvGroup(ctx, g.ID); !errors.Is(err, core.ErrConflict) {
				close(kv.resume)
				<-first
				t.Fatalf("delete during content effects = %v, want conflict", err)
			}
			if _, err := replica.SetEnvGroupVar(ctx, independent.ID, "OTHER", "independent"); err != nil {
				close(kv.resume)
				<-first
				t.Fatalf("another group was serialized behind this one: %v", err)
			}
			close(kv.resume)
			if err := <-first; err != nil {
				t.Fatalf("original instance = %v", err)
			}
			if _, err := replica.SetEnvGroupVar(ctx, g.ID, "TOKEN", "newest"); err != nil {
				t.Fatal(err)
			}
			// A recoverer that read its row before either owner finished must
			// re-read after taking the lock; its old snapshot cannot restore maps.
			if err := writer.withGroupContent(ctx, stale.ID, func(context.Context, store.EnvGroup) error { return nil }); err != nil {
				t.Fatal(err)
			}
			value, err := replica.GetEnvGroupVar(ctx, g.ID, "TOKEN")
			if err != nil || value.Value != "newest" || string(secret(t, writer.Client, envSecretName(g.ID)).Data["TOKEN"]) != "newest" {
				t.Fatalf("late recovery replaced the newest source/projection: %+v, %v", value, err)
			}
		})
	}
}
