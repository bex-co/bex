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

// Package migrate copies one App's legacy registry tags and static objects
// onto the workspace-scoped identity (w2/m75, docs/ADR074). Dry-run is the
// default: Apply is the only mutating entry. A digest/ETag mismatch aborts
// that App and leaves the legacy location authoritative. Tombstone never
// deletes blobs.
package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/identity"
)

const tombstoneTag = identity.TombstoneTag

// Registry is the OCI operations the planner needs. Tests fake it; the CLI
// wraps skopeo + the distribution HTTP API.
type Registry interface {
	ListTags(ctx context.Context, repo string) ([]string, error)
	Digest(ctx context.Context, repo, tag string) (string, error)
	CopyTag(ctx context.Context, srcRepo, dstRepo, tag string) error
	PutTombstone(ctx context.Context, repo, digest string) error
}

// Objects is the S3 operations the planner needs.
type Objects interface {
	List(ctx context.Context, prefix string) ([]ObjectMeta, error)
	Head(ctx context.Context, key string) (ObjectMeta, error)
	Copy(ctx context.Context, srcKey, dstKey string) error
	PutTombstone(ctx context.Context, key string, body []byte) error
}

// ObjectMeta is the verification tuple for one object (ETag + size).
type ObjectMeta struct {
	Key  string
	ETag string
	Size int64
}

// TagStep is one registry tag copy.
type TagStep struct {
	Tag       string
	SrcDigest string
	DstDigest string
	Skip      bool // destination already matches
}

// ObjectStep is one object copy (key relative to the app prefix).
type ObjectStep struct {
	RelPath string
	Src     ObjectMeta
	Dst     ObjectMeta
	Skip    bool
}

// Plan is the exact copy/verify/tombstone work for one App. Dry-run prints it;
// Apply executes it.
type Plan struct {
	App       string
	Namespace string
	ID        identity.Identity
	Tags      []TagStep
	Objects   []ObjectStep
	Tombstone bool // true when a tombstone should be written after verify
	Notes     []string
}

// AppRef is the cluster-wide identity a sibling check needs (UID + name +
// workspace label). The CLI supplies this from a List of Apps.
type AppRef struct {
	UID, Name, Workspace string
}

// SiblingOwnsLegacy reports whether another live App still keys the same
// legacy repository/prefix. Tombstone is refused in that case; copy to the
// destination may still proceed.
func SiblingOwnsLegacy(apps []AppRef, currentUID string, id identity.Identity) bool {
	legacy := id.LegacyRepo()
	for _, other := range apps {
		if other.UID == currentUID {
			continue
		}
		oid := identity.ForApp(other.Name, other.Workspace)
		if oid.Repo() == legacy || (!oid.Scoped() && oid.LegacyRepo() == legacy) {
			return true
		}
	}
	return false
}

// Engine binds the two sinks.
type Engine struct {
	Registry Registry
	Objects  Objects
}

// BuildPlan enumerates legacy artifacts and the destination state. It mutates
// nothing. skipTombstone is set when a sibling still owns the legacy location.
func (e *Engine) BuildPlan(ctx context.Context, app, namespace string, id identity.Identity, skipTombstone bool) (Plan, error) {
	p := Plan{App: app, Namespace: namespace, ID: id, Tombstone: !skipTombstone}
	if !id.Scoped() {
		return p, fmt.Errorf("migration requires a workspace id (app %q has none)", app)
	}
	srcRepo, dstRepo := id.LegacyRepo(), id.Repo()
	tags, err := e.Registry.ListTags(ctx, srcRepo)
	if err != nil {
		return p, fmt.Errorf("list legacy repo %s: %w", srcRepo, err)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		if tag == tombstoneTag {
			p.Notes = append(p.Notes, "legacy repo already tombstoned")
			continue
		}
		srcDigest, err := e.Registry.Digest(ctx, srcRepo, tag)
		if err != nil {
			return p, fmt.Errorf("digest %s:%s: %w", srcRepo, tag, err)
		}
		step := TagStep{Tag: tag, SrcDigest: srcDigest}
		if dstDigest, err := e.Registry.Digest(ctx, dstRepo, tag); err == nil {
			step.DstDigest = dstDigest
			if dstDigest == srcDigest {
				step.Skip = true
			}
		}
		p.Tags = append(p.Tags, step)
	}

	if e.Objects != nil {
		srcPrefix := id.LegacyStaticAppPrefix() + "/"
		objs, err := e.Objects.List(ctx, srcPrefix)
		if err != nil {
			return p, fmt.Errorf("list legacy prefix %s: %w", srcPrefix, err)
		}
		dstRoot := id.StaticAppPrefix() + "/"
		for _, obj := range objs {
			if obj.Key == srcPrefix+identity.TombstoneObject {
				p.Notes = append(p.Notes, "legacy prefix already tombstoned")
				continue
			}
			rel := strings.TrimPrefix(obj.Key, srcPrefix)
			step := ObjectStep{RelPath: rel, Src: obj}
			dstKey := dstRoot + rel
			if dst, err := e.Objects.Head(ctx, dstKey); err == nil {
				step.Dst = dst
				if dst.ETag == obj.ETag && dst.Size == obj.Size {
					step.Skip = true
				}
			}
			p.Objects = append(p.Objects, step)
		}
	}
	if skipTombstone {
		p.Notes = append(p.Notes, "tombstone skipped: another App still keys the legacy location")
	}
	return p, nil
}

// Verify reports whether every non-skipped step has a matching destination
// digest/ETag. A mismatch is a hard failure: the caller must not tombstone.
func (p Plan) Verify() error {
	for _, t := range p.Tags {
		if t.Skip {
			continue
		}
		if t.DstDigest == "" {
			return fmt.Errorf("tag %s: destination missing", t.Tag)
		}
		if t.DstDigest != t.SrcDigest {
			return fmt.Errorf("tag %s: digest mismatch src=%s dst=%s", t.Tag, t.SrcDigest, t.DstDigest)
		}
	}
	for _, o := range p.Objects {
		if o.Skip {
			continue
		}
		if o.Dst.Key == "" && o.Dst.ETag == "" {
			return fmt.Errorf("object %s: destination missing", o.RelPath)
		}
		if o.Dst.ETag != o.Src.ETag || o.Dst.Size != o.Src.Size {
			return fmt.Errorf("object %s: etag/size mismatch src=%s/%d dst=%s/%d",
				o.RelPath, o.Src.ETag, o.Src.Size, o.Dst.ETag, o.Dst.Size)
		}
	}
	return nil
}

// conflicts is the pre-copy half of Verify: a destination that already holds
// a different digest/ETag must not be overwritten.
func (p Plan) conflicts() error {
	for _, t := range p.Tags {
		if t.DstDigest != "" && t.DstDigest != t.SrcDigest {
			return fmt.Errorf("tag %s: destination already has different digest src=%s dst=%s", t.Tag, t.SrcDigest, t.DstDigest)
		}
	}
	for _, o := range p.Objects {
		if o.Dst.ETag == "" {
			continue
		}
		if o.Dst.ETag != o.Src.ETag || o.Dst.Size != o.Src.Size {
			return fmt.Errorf("object %s: etag/size mismatch src=%s/%d dst=%s/%d",
				o.RelPath, o.Src.ETag, o.Src.Size, o.Dst.ETag, o.Dst.Size)
		}
	}
	return nil
}

// Apply copies missing tags/objects, re-reads destinations, verifies, then
// optionally tombstones. Re-running after a partial copy converges. On verify
// failure the legacy location is left authoritative (no tombstone). A
// destination that already holds a different digest/ETag is a hard conflict:
// Apply mutates nothing.
func (e *Engine) Apply(ctx context.Context, p Plan) (Plan, error) {
	if err := p.conflicts(); err != nil {
		return p, fmt.Errorf("verify failed (legacy location still authoritative): %w", err)
	}
	id := p.ID
	srcRepo, dstRepo := id.LegacyRepo(), id.Repo()
	for i, t := range p.Tags {
		if t.Skip {
			continue
		}
		if err := e.Registry.CopyTag(ctx, srcRepo, dstRepo, t.Tag); err != nil {
			return p, fmt.Errorf("copy %s:%s: %w", srcRepo, t.Tag, err)
		}
		dst, err := e.Registry.Digest(ctx, dstRepo, t.Tag)
		if err != nil {
			return p, fmt.Errorf("re-read %s:%s: %w", dstRepo, t.Tag, err)
		}
		p.Tags[i].DstDigest = dst
	}
	if e.Objects != nil {
		srcPrefix := id.LegacyStaticAppPrefix() + "/"
		dstRoot := id.StaticAppPrefix() + "/"
		for i, o := range p.Objects {
			if o.Skip {
				continue
			}
			srcKey := srcPrefix + o.RelPath
			dstKey := dstRoot + o.RelPath
			if err := e.Objects.Copy(ctx, srcKey, dstKey); err != nil {
				return p, fmt.Errorf("copy object %s: %w", o.RelPath, err)
			}
			dst, err := e.Objects.Head(ctx, dstKey)
			if err != nil {
				return p, fmt.Errorf("re-read object %s: %w", o.RelPath, err)
			}
			p.Objects[i].Dst = dst
		}
	}
	if err := p.Verify(); err != nil {
		return p, fmt.Errorf("verify failed (legacy location still authoritative): %w", err)
	}
	if !p.Tombstone {
		return p, nil
	}
	digest := ""
	for _, t := range p.Tags {
		if t.SrcDigest != "" {
			digest = t.SrcDigest
			break
		}
	}
	if digest != "" {
		if err := e.Registry.PutTombstone(ctx, srcRepo, digest); err != nil {
			return p, fmt.Errorf("registry tombstone: %w", err)
		}
	}
	if e.Objects != nil {
		body, _ := json.Marshal(map[string]string{
			"migratedTo": id.StaticAppPrefix() + "/",
			"at":         time.Now().UTC().Format(time.RFC3339),
		})
		key := id.LegacyStaticAppPrefix() + "/" + identity.TombstoneObject
		if err := e.Objects.PutTombstone(ctx, key, body); err != nil {
			return p, fmt.Errorf("s3 tombstone: %w", err)
		}
	}
	return p, nil
}

// Format prints the dry-run plan.
func (p Plan) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "app %s/%s workspace=%s repo %s -> %s prefix %s -> %s\n",
		p.Namespace, p.App, p.ID.Workspace, p.ID.LegacyRepo(), p.ID.Repo(),
		p.ID.LegacyStaticAppPrefix()+"/", p.ID.StaticAppPrefix()+"/")
	for _, t := range p.Tags {
		action := "copy"
		if t.Skip {
			action = "skip (digest match)"
		}
		fmt.Fprintf(&b, "  tag %s %s src=%s dst=%s\n", t.Tag, action, t.SrcDigest, t.DstDigest)
	}
	for _, o := range p.Objects {
		action := "copy"
		if o.Skip {
			action = "skip (etag match)"
		}
		fmt.Fprintf(&b, "  obj %s %s src=%s/%d dst=%s/%d\n", o.RelPath, action, o.Src.ETag, o.Src.Size, o.Dst.ETag, o.Dst.Size)
	}
	if p.Tombstone {
		b.WriteString("  tombstone: yes (no blob delete)\n")
	} else {
		b.WriteString("  tombstone: no\n")
	}
	for _, n := range p.Notes {
		fmt.Fprintf(&b, "  note: %s\n", n)
	}
	return b.String()
}
