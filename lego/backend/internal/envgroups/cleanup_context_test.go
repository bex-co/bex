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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type cleanupProbeKey struct{}

type cleanupRecordingGroups struct {
	GroupStore
	observe func(context.Context)
}

func (s *cleanupRecordingGroups) ReleaseEnvGroupContent(ctx context.Context, gid string, version int64, rel store.EnvGroupRelease) error {
	s.observe(ctx)
	return s.GroupStore.ReleaseEnvGroupContent(ctx, gid, version, rel)
}

type cleanupRecordingKV struct {
	*fakeStore
	request   context.Context
	cancel    context.CancelFunc
	failFiles bool
	observe   func(context.Context)
}

func (s *cleanupRecordingKV) Put(ctx context.Context, path string, values map[string]string) error {
	if s.failFiles && strings.HasSuffix(path, "/files") {
		s.failFiles = false
		s.cancel()
		return errors.New("live files write refused after caller cancellation")
	}
	if s.request.Err() != nil {
		s.observe(ctx)
	}
	return s.fakeStore.Put(ctx, path, values)
}

func (s *cleanupRecordingKV) Delete(ctx context.Context, path string) error {
	if s.request.Err() != nil {
		s.observe(ctx)
	}
	return s.fakeStore.Delete(ctx, path)
}

func TestCancellationCleanupSharesOneDeadlineAcrossMapsAndRowRelease(t *testing.T) {
	for _, compensate := range []bool{false, true} {
		name := "committed"
		if compensate {
			name = "compensated"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.WithValue(testCtx(), cleanupProbeKey{}, "preserved"))
			defer cancel()
			var deadline time.Time
			observations := 0
			observe := func(cleanup context.Context) {
				t.Helper()
				actual, bounded := cleanup.Deadline()
				if cleanup.Err() != nil || !bounded || cleanup.Value(cleanupProbeKey{}) != "preserved" {
					t.Fatalf("cleanup did not detach with a bound and its request values: canceled=%v bounded=%v", cleanup.Err(), bounded)
				}
				if observations > 0 && !actual.Equal(deadline) {
					t.Fatal("nested cleanup renewed or removed the original cleanup deadline")
				}
				deadline = actual
				observations++
			}
			kv := &cleanupRecordingKV{fakeStore: newFakeStore(), request: ctx, cancel: cancel, observe: observe}
			svc := newService(kv)
			g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
				SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "before"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			svc.Groups = &cleanupRecordingGroups{GroupStore: svc.Groups, observe: func(cleanup context.Context) {
				observe(cleanup)
				cancel()
			}}
			kv.failFiles = compensate
			_, err = svc.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
				ExpectedRevision: &g.Revision, SaveMode: SaveModeOnly,
				EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "after"}}, SecretFiles: []SecretFilePatch{{Name: "cert.pem", Content: "after"}},
			})
			if compensate {
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Code != "ENV_GROUP_UPDATE_RESTORED" {
					t.Fatalf("canceled compensation = %v", err)
				}
			} else if err != nil {
				t.Fatalf("committed save after caller cancellation = %v", err)
			}
			if observations < 6 {
				t.Fatalf("observed %d cleanup effects, want row releases and operation-map cleanup", observations)
			}
		})
	}
}
