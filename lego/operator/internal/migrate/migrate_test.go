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

package migrate

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/identity"
)

type fakeReg struct {
	tags    map[string]map[string]string // repo -> tag -> digest
	copied  int
	tomb    map[string]string
	failTag string
}

func newFakeReg() *fakeReg {
	return &fakeReg{tags: map[string]map[string]string{}, tomb: map[string]string{}}
}

func (f *fakeReg) ListTags(_ context.Context, repo string) ([]string, error) {
	tags := f.tags[repo]
	out := make([]string, 0, len(tags))
	for t := range tags {
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeReg) Digest(_ context.Context, repo, tag string) (string, error) {
	d, ok := f.tags[repo][tag]
	if !ok {
		return "", fmt.Errorf("missing %s:%s", repo, tag)
	}
	return d, nil
}

func (f *fakeReg) CopyTag(_ context.Context, srcRepo, dstRepo, tag string) error {
	if tag == f.failTag {
		return fmt.Errorf("injected copy failure")
	}
	src, ok := f.tags[srcRepo][tag]
	if !ok {
		return fmt.Errorf("src missing %s:%s", srcRepo, tag)
	}
	if f.tags[dstRepo] == nil {
		f.tags[dstRepo] = map[string]string{}
	}
	f.tags[dstRepo][tag] = src
	f.copied++
	return nil
}

func (f *fakeReg) PutTombstone(_ context.Context, repo, digest string) error {
	if f.tags[repo] == nil {
		f.tags[repo] = map[string]string{}
	}
	f.tags[repo][tombstoneTag] = digest
	f.tomb[repo] = digest
	return nil
}

type fakeObj struct {
	objs  map[string]ObjectMeta
	tomb  string
	fail  string
	copyN int
}

func newFakeObj() *fakeObj { return &fakeObj{objs: map[string]ObjectMeta{}} }

func (f *fakeObj) List(_ context.Context, prefix string) ([]ObjectMeta, error) {
	var out []ObjectMeta
	for k, v := range f.objs {
		if strings.HasPrefix(k, prefix) {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeObj) Head(_ context.Context, key string) (ObjectMeta, error) {
	v, ok := f.objs[key]
	if !ok {
		return ObjectMeta{}, fmt.Errorf("missing %s", key)
	}
	return v, nil
}

func (f *fakeObj) Copy(_ context.Context, srcKey, dstKey string) error {
	if srcKey == f.fail {
		return fmt.Errorf("injected copy failure")
	}
	src, ok := f.objs[srcKey]
	if !ok {
		return fmt.Errorf("src missing %s", srcKey)
	}
	dst := src
	dst.Key = dstKey
	f.objs[dstKey] = dst
	f.copyN++
	return nil
}

func (f *fakeObj) PutTombstone(_ context.Context, key string, _ []byte) error {
	f.tomb = key
	f.objs[key] = ObjectMeta{Key: key, ETag: "tomb", Size: 1}
	return nil
}

func testID() identity.Identity {
	return identity.ForApp("web", "tea-aaaaaaaaaaaaaaaaaaaa")
}

func TestDryRunBuildPlanMutatesNothing(t *testing.T) {
	reg := newFakeReg()
	reg.tags["web"] = map[string]string{"gen-1": "sha256:aaa"}
	obj := newFakeObj()
	obj.objs["web/rev-1/index.html"] = ObjectMeta{Key: "web/rev-1/index.html", ETag: "e1", Size: 10}
	e := &Engine{Registry: reg, Objects: obj}
	p, err := e.BuildPlan(context.Background(), "web", "default", testID(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tags) != 1 || p.Tags[0].SrcDigest != "sha256:aaa" || p.Tags[0].Skip {
		t.Fatalf("tags = %+v", p.Tags)
	}
	if len(p.Objects) != 1 || p.Objects[0].Skip {
		t.Fatalf("objects = %+v", p.Objects)
	}
	if reg.copied != 0 || obj.copyN != 0 || len(reg.tomb) != 0 || obj.tomb != "" {
		t.Fatal("BuildPlan mutated state")
	}
	text := p.Format()
	if !strings.Contains(text, "web -> tea-aaaaaaaaaaaaaaaaaaaa/web") {
		t.Errorf("plan missing repo mapping: %s", text)
	}
}

func TestVerifyMismatchAbortsWithoutTombstone(t *testing.T) {
	reg := newFakeReg()
	reg.tags["web"] = map[string]string{"gen-1": "sha256:aaa"}
	reg.tags["tea-aaaaaaaaaaaaaaaaaaaa/web"] = map[string]string{"gen-1": "sha256:bbb"}
	e := &Engine{Registry: reg}
	p, err := e.BuildPlan(context.Background(), "web", "default", testID(), false)
	if err != nil {
		t.Fatal(err)
	}
	// Destination already has a different digest — Apply must not copy over it
	// blindly; Verify fails and tombstone is not written.
	p.Tags[0].Skip = false
	if _, err := e.Apply(context.Background(), p); err == nil {
		t.Fatal("expected verify failure")
	}
	if reg.copied != 0 {
		t.Fatal("mismatch must not overwrite the destination")
	}
	if _, ok := reg.tomb["web"]; ok {
		t.Fatal("tombstone written after verify failure")
	}
	if got := reg.tags["web"]["gen-1"]; got != "sha256:aaa" {
		t.Fatalf("legacy digest changed: %s", got)
	}
	if got := reg.tags["tea-aaaaaaaaaaaaaaaaaaaa/web"]["gen-1"]; got != "sha256:bbb" {
		t.Fatalf("destination digest changed: %s", got)
	}
}

func TestApplyIdempotentResume(t *testing.T) {
	reg := newFakeReg()
	reg.tags["web"] = map[string]string{"gen-1": "sha256:aaa", "gen-2": "sha256:ccc"}
	obj := newFakeObj()
	obj.objs["web/rev-1/index.html"] = ObjectMeta{Key: "web/rev-1/index.html", ETag: "e1", Size: 10}
	e := &Engine{Registry: reg, Objects: obj}

	id := testID()
	p, err := e.BuildPlan(context.Background(), "web", "default", id, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Apply(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	firstCopies := reg.copied + obj.copyN
	if firstCopies == 0 {
		t.Fatal("first apply copied nothing")
	}
	if _, ok := reg.tomb["web"]; !ok {
		t.Fatal("expected registry tombstone")
	}
	if obj.tomb == "" {
		t.Fatal("expected s3 tombstone")
	}

	p2, err := e.BuildPlan(context.Background(), "web", "default", id, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tstep := range p2.Tags {
		if tstep.Tag == tombstoneTag {
			continue
		}
		if !tstep.Skip {
			t.Errorf("tag %s not skipped on resume", tstep.Tag)
		}
	}
	for _, o := range p2.Objects {
		if o.RelPath == identity.TombstoneObject {
			continue
		}
		if !o.Skip {
			t.Errorf("object %s not skipped on resume", o.RelPath)
		}
	}
	copiedBefore := reg.copied + obj.copyN
	if _, err := e.Apply(context.Background(), p2); err != nil {
		t.Fatal(err)
	}
	if reg.copied+obj.copyN != copiedBefore {
		t.Fatalf("resume re-copied: before=%d after=%d", copiedBefore, reg.copied+obj.copyN)
	}
}

func TestUnscopedPlanErrors(t *testing.T) {
	e := &Engine{Registry: newFakeReg()}
	_, err := e.BuildPlan(context.Background(), "web", "default", identity.ForApp("web", ""), false)
	if err == nil {
		t.Fatal("expected error for unlabeled App")
	}
}

func TestSkipTombstoneWhenSiblingOwnsLegacy(t *testing.T) {
	id := testID()
	if !SiblingOwnsLegacy([]AppRef{
		{UID: "uid-self", Name: "web", Workspace: id.Workspace},
		{UID: "uid-sib", Name: "web", Workspace: ""},
	}, "uid-self", id) {
		t.Fatal("unlabeled sibling must block tombstone")
	}
	if SiblingOwnsLegacy([]AppRef{
		{UID: "uid-self", Name: "web", Workspace: id.Workspace},
		{UID: "uid-other", Name: "other", Workspace: ""},
	}, "uid-self", id) {
		t.Fatal("disjoint sibling must not block tombstone")
	}

	reg := newFakeReg()
	reg.tags["web"] = map[string]string{"gen-1": "sha256:aaa"}
	e := &Engine{Registry: reg}
	p, err := e.BuildPlan(context.Background(), "web", "default", id, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.Tombstone {
		t.Fatal("plan must not tombstone when a sibling owns the legacy repo")
	}
	if _, err := e.Apply(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.tomb["web"]; ok {
		t.Fatal("sibling-owned legacy repo was tombstoned")
	}
	if got := reg.tags["web"]["gen-1"]; got != "sha256:aaa" {
		t.Fatalf("legacy blob deleted or changed: %s", got)
	}
}

func TestApplyLeavesLegacyBlobs(t *testing.T) {
	reg := newFakeReg()
	reg.tags["web"] = map[string]string{"gen-1": "sha256:aaa"}
	e := &Engine{Registry: reg}
	p, err := e.BuildPlan(context.Background(), "web", "default", testID(), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Apply(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if got := reg.tags["web"]["gen-1"]; got != "sha256:aaa" {
		t.Fatalf("legacy tag missing after tombstone: %s", got)
	}
	if _, ok := reg.tomb["web"]; !ok {
		t.Fatal("expected tombstone marker tag")
	}
}

func TestBuildPlanCopiesTenantTombstoneNamedFiles(t *testing.T) {
	id := testID()
	obj := newFakeObj()
	obj.objs["web/.bex-tombstone"] = ObjectMeta{Key: "web/.bex-tombstone", ETag: "marker", Size: 4}
	obj.objs["web/dir/.bex-tombstone"] = ObjectMeta{Key: "web/dir/.bex-tombstone", ETag: "e-nested", Size: 8}
	obj.objs["web/photo.bex-tombstone"] = ObjectMeta{Key: "web/photo.bex-tombstone", ETag: "e-file", Size: 16}
	obj.objs["web/rev-1/index.html"] = ObjectMeta{Key: "web/rev-1/index.html", ETag: "e1", Size: 10}
	e := &Engine{Registry: newFakeReg(), Objects: obj}
	p, err := e.BuildPlan(context.Background(), "web", "default", id, false)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range p.Objects {
		got[o.RelPath] = true
	}
	if got[identity.TombstoneObject] {
		t.Fatal("prefix-root tombstone marker was planned as a tenant object")
	}
	if !got["dir/.bex-tombstone"] || !got["photo.bex-tombstone"] || !got["rev-1/index.html"] {
		t.Fatalf("tenant objects dropped: %+v", got)
	}
}
