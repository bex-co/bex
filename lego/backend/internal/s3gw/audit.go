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

package s3gw

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Audit rows of the S3 surface (ADR112 D6). A write relation records its allowed and its
// denied call at authorization; a read records only a denial. Three S3 cases need more:
// a successful get records one row when the object opened (and one per access key,
// bucket and key per 15 minutes on a replica, because `aws s3 cp` reads in 8 MB ranges
// ten at a time), a denied row is written at most once per access key, verb and bucket
// in the same window (a leaked read key cannot fill the log with denied puts), and a
// DeleteObjects records one row naming the bucket, the count and the first key.

// AuditWindow is how long a get or a denial writes one row.
const AuditWindow = 15 * time.Minute

// onceWindow answers true the first time a key is seen in a window, and bounds its memory
// by sweeping the expired and then resetting, as core.TTLCache does.
type onceWindow struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func newOnceWindow() *onceWindow { return &onceWindow{seen: map[string]time.Time{}} }

const onceWindowMax = 8192

func (o *onceWindow) first(key string, now time.Time) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if at, ok := o.seen[key]; ok && now.Sub(at) < AuditWindow {
		return false
	}
	if len(o.seen) >= onceWindowMax {
		for k, at := range o.seen {
			if now.Sub(at) >= AuditWindow {
				delete(o.seen, k)
			}
		}
		if len(o.seen) >= onceWindowMax {
			o.seen = map[string]time.Time{}
		}
	}
	o.seen[key] = now
	return true
}

// recordGet writes the download row once per access key, bucket and key in the window.
func (rq *request) recordGet(b store.ObjectBucket) {
	if !rq.g.views.first(rq.key.ID+"\x00"+b.ID+"\x00"+rq.objectKey, rq.g.cfg.Now()) {
		return
	}
	rq.g.cfg.Base.RecordObjectViewed(rq.ctx, rq.workspaceID, core.ObjectTarget(b.Name, rq.objectKey))
}

// recordDeleteObjects writes the one row of a batch delete: the bucket, how many keys and
// the first.
func (rq *request) recordDeleteObjects(b store.ObjectBucket, count int, first string) {
	rq.g.cfg.Base.RecordAllowedWrite(rq.ctx, objectstorage.AuditVerbDeleteObjects, core.RelCanCreate, rq.workspaceID,
		core.ObjectTarget(b.Name, first), fmt.Sprintf("%d keys", count))
}

// denialDedupe is the audit sink the S3 surface gives its Base: it passes everything on,
// but writes a denied row of an S3 key at most once per key, verb and bucket in the
// window, so a leaked read key hammering put cannot fill a workspace's log. Allowed rows
// and every other caller's rows are untouched.
type denialDedupe struct {
	inner core.AuditSink
	once  *onceWindow
	now   func() time.Time
}

// repeat reports a denied row of an S3 key that the window already holds.
func (d *denialDedupe) repeat(ev core.AuditEvent) bool {
	return ev.Outcome == core.AuditDenied && ev.CallerMethod == core.MethodS3Key &&
		!d.once.first(ev.Caller+"\x00"+ev.Verb+"\x00"+targetBucket(ev.Target), d.now())
}

func (d *denialDedupe) Record(ctx context.Context, ev core.AuditEvent) error {
	if d.repeat(ev) {
		return nil
	}
	return d.inner.Record(ctx, ev)
}

// RecordAll keeps a batch sink a batch sink: the Base writes rows that record one fact
// together when its sink can.
func (d *denialDedupe) RecordAll(ctx context.Context, evs []core.AuditEvent) error {
	kept := make([]core.AuditEvent, 0, len(evs))
	for _, ev := range evs {
		if !d.repeat(ev) {
			kept = append(kept, ev)
		}
	}
	if batch, ok := d.inner.(core.AuditBatchSink); ok {
		return batch.RecordAll(ctx, kept)
	}
	for _, ev := range kept {
		if err := d.inner.Record(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// targetBucket is the bucket an audit target names: "bucket:<name>" or
// "object:<bucket>/<key>".
func targetBucket(target string) string {
	_, rest, _ := strings.Cut(target, ":")
	name, _, _ := strings.Cut(rest, "/")
	return name
}

// recordCeilingDenial writes the denied row of a request its key's permission refused
// before OpenFGA was asked: the key as the actor, the workspace as the resource.
func (g *Gateway) recordCeilingDenial(rq *request, relation, verb, target string) {
	if g.cfg.Base == nil || g.cfg.Base.Audit == nil {
		return
	}
	core.RecordAuditEvent(rq.ctx, g.cfg.Base.Audit, core.AuditEvent{
		Caller: rq.key.ID, CallerMethod: core.MethodS3Key, Verb: verb, Resource: core.WorkspaceObject(rq.workspaceID),
		Target: target, Relation: relation, Outcome: core.AuditDenied, At: g.cfg.Now(),
	})
}
