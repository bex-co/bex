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

package logs

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
)

// project.go reads a project's log view (w3/m505, docs/ADR010-observability.md
// § Project views): the scope a view's filter document selects among the
// project's resources, resolved on the server on every read and every tail
// re-resolution, and read like a multi-resource `resource` query.

// ProjectResources lists a project's resources for its log views, to a
// caller who may view the project; projects.Service satisfies it. The
// workspace it answers is the project's.
type ProjectResources interface {
	ProjectLogResources(ctx context.Context, projectID string) (workspaceID string, resources []savedsearch.Resource, err error)
}

// AuditVerbQueryLogs is the verb a project view's reads record: each is a
// QueryLogs of one resource.
const AuditVerbQueryLogs = "logs.QueryLogs"

// ErrProjectLogsUnavailable answers a project-scoped read when no project
// listing is wired (no control-plane store).
var ErrProjectLogsUnavailable = core.Unavailable("project log views need the projects store")

// maxProjectLogResources bounds one project query's fan-out: REST's cap on
// the `resource` array, so a view reads no more than a REST caller may.
const maxProjectLogResources = maxLogResources

// projectLogPageSize is Render's page for a project view (pageSize 50).
const projectLogPageSize = 50

// DefaultProjectRescopeInterval is how often a project tail re-resolves its
// scope, so a resource created in or moved into the project joins it.
const DefaultProjectRescopeInterval = 30 * time.Second

// projectView is a project-scoped read: the view type its log type names, the
// filter document, and the project.
type projectView struct {
	project  string
	viewType string
	doc      savedsearch.Document
}

// newProjectView reads a project-scoped query: its log type picks the view
// (app → Application Logs, the default; request → Request Logs), the filter
// document (nil = Render's empty default) is validated for that view, and its
// line filters join q, an IS_NOT one as a negated filter (w3/m523). A filter
// the caller sets beside the same field in the document is refused by name
// rather than silently widened or narrowed, whichever operator the view uses.
func newProjectView(project string, rawDoc any, q *LogQuery) (projectView, error) {
	v := projectView{project: project, viewType: savedsearch.TypeApplicationLogs}
	switch {
	case len(q.Types) == 0 || slices.Equal(q.Types, []string{LogTypeApp}):
		q.Types = []string{LogTypeApp}
	case slices.Equal(q.Types, []string{LogTypeRequest}):
		v.viewType = savedsearch.TypeRequestLogs
	default:
		return projectView{}, fmt.Errorf("%w: a project log view reads type=%s or type=%s, not %v", core.ErrBadRequest, LogTypeApp, LogTypeRequest, q.Types)
	}
	if len(q.Build) > 0 || q.Deploy != "" {
		return projectView{}, fmt.Errorf("%w: a project log view takes no build or deploy filter", core.ErrBadRequest)
	}
	v.doc = savedsearch.Default()
	if rawDoc != nil {
		doc, err := savedsearch.ParseValue(rawDoc, v.viewType)
		if err != nil {
			return projectView{}, err
		}
		v.doc = doc
	}
	lineFilters := []struct {
		field string
		label string // the filter's name in q.negated
		into  *[]string
	}{
		{savedsearch.FieldLevel, LabelLevel, &q.Level},
		{savedsearch.FieldInstance, LabelInstance, &q.Instance},
		{savedsearch.FieldText, "", &q.Search}, // Validate refuses IS_NOT on TEXT
		{savedsearch.FieldMethod, LabelMethod, &q.Method},
		{savedsearch.FieldStatusCode, LabelStatusCode, &q.StatusCode},
		{savedsearch.FieldHost, LabelHost, &q.Host},
		{savedsearch.FieldPath, LabelPath, &q.Path},
	}
	for _, f := range lineFilters {
		values, negated := v.doc.Values(f.field)
		if len(values) == 0 {
			continue
		}
		if len(*f.into) > 0 {
			return projectView{}, fmt.Errorf("%w: the view already filters on %s; edit the view instead of adding the filter", core.ErrBadRequest, f.field)
		}
		*f.into = slices.Clone(values)
		if negated {
			q.negated = append(q.negated, f.label)
		}
	}
	q.projectScope = true
	return v, nil
}

// resolveProjectView is the view's scope as the caller may read it: the project's
// resources the document keeps, each one whose logs the caller may read. A
// caller without log access to the workspace reads an empty scope, decided by
// one quiet probe so the denial is not audited once per resource; a resource
// that then refuses or is gone is dropped. Neither is ever named. A scope
// above maxProjectLogResources is refused, and Request Logs answers the
// workspace plan's request-log rule first.
//
// A resource in followed is one a live tail already follows: its follower
// re-checks its access on its own watchdog, so a re-resolution keeps it
// without authorizing it again.
func (s *Service) resolveProjectView(ctx context.Context, v projectView, followed map[string]bool) (string, []savedsearch.Resource, error) {
	if s.Projects == nil {
		return "", nil, ErrProjectLogsUnavailable
	}
	workspace, all, err := s.Projects.ProjectLogResources(ctx, v.project)
	if err != nil {
		return "", nil, err
	}
	if v.viewType == savedsearch.TypeRequestLogs {
		wp, err := s.workspacePlanOf(ctx, workspace)
		if err != nil {
			return "", nil, err
		}
		if wp.ok && !core.PlanAllows(wp.plan, core.FeatureRequestLogs) {
			return "", nil, core.NewPlanFeatureError("type", wp.plan, core.FeatureRequestLogs)
		}
	}
	scoped := v.doc.Scope(v.viewType, all)
	if len(scoped) > maxProjectLogResources {
		return "", nil, fmt.Errorf("%w: this view reads %d resources, and one project log query reads at most %d; narrow it with an Environment, Resource or Resource Type filter", core.ErrBadRequest, len(scoped), maxProjectLogResources)
	}
	if !s.CanDecisionOn(ctx, core.RelCanViewLogs, core.WorkspaceObject(workspace)).Allowed() {
		return workspace, []savedsearch.Resource{}, nil
	}
	// The resources authorize at once, as a page reads them (readEach), each
	// under the verb of the read that asked.
	ctx = core.WithAuditVerb(ctx, core.CurrentVerb(ctx))
	verdicts := make([]error, len(scoped))
	var g errgroup.Group
	g.SetLimit(maxConcurrentLogReads)
	for i, r := range scoped {
		if followed[r.ID] {
			continue
		}
		g.Go(func() error {
			_, verdicts[i] = s.authorizeLogOwner(ctx, r.ID)
			return nil
		})
	}
	_ = g.Wait()
	readable := []savedsearch.Resource{}
	for i, r := range scoped {
		switch err := verdicts[i]; {
		case err == nil:
			readable = append(readable, r)
		case errors.Is(err, core.ErrForbidden) || errors.Is(err, core.ErrNotFound):
		default:
			return "", nil, err
		}
	}
	return workspace, readable, nil
}

// forResource is q pointed at one resource. A project view's Application Logs
// read a datastore's process log, which carries no log type (its own
// contract refuses one), so the view's type filter stays off it.
func (q LogQuery) forResource(id string) LogQuery {
	q.App = id
	if q.projectScope && core.IsDatastoreResource(id) {
		q.Types = nil
	}
	return q
}

// queryProjectLogPage reads one page of a project view, and the resources it
// read: what LogList.resources names.
func (s *Service) queryProjectLogPage(ctx context.Context, project string, rawDoc any, q LogQuery) (logPage, []savedsearch.Resource, error) {
	// Reached from the GraphQL resolver's closure: the reads record as the
	// resource read they are.
	ctx = core.WithRequestMemo(core.WithAuditVerb(ctx, AuditVerbQueryLogs))
	v, err := newProjectView(project, rawDoc, &q)
	if err != nil {
		return logPage{}, nil, err
	}
	if q.Limit == 0 {
		q.Limit = projectLogPageSize
	}
	if err := q.validate(); err != nil {
		return logPage{}, nil, err
	}
	_, resources, err := s.resolveProjectView(ctx, v, nil)
	if err != nil {
		return logPage{}, nil, err
	}
	if len(resources) == 0 {
		n := q.normalized()
		page := logPage{Entries: []LogEntry{}}
		page.NextStartTime, page.NextEndTime = pageCursors(nil, n.Since, n.End, n.Direction)
		return page, resources, nil
	}
	page, err := s.queryLogPage(ctx, resourceIDs(resources), q)
	return page, resources, err
}

// AuditVerbLogLabelValues is the verb a project view's value discovery
// records: each read is a LogLabelValues of one resource.
const AuditVerbLogLabelValues = "logs.LogLabelValues"

// queryProjectLabelValues lists the values one label takes across a project
// view's resources (w3/079): Render's logValues(input: {label, logQuery:
// {project, …}}), which feeds its Edit view's value pickers. The scope is the
// view's, resolved and authorized as a page read resolves it, so it reads at
// most maxProjectLogResources resources and names none the caller may not
// read. Each resource then answers its own LogLabelValues, eight at a time,
// so an instance keeps its resource's id as its prefix. A datastore, whose
// lines carry only an instance and a level, adds nothing to another label,
// and a resource that refuses or is gone by its read adds nothing either.
func (s *Service) queryProjectLabelValues(ctx context.Context, project string, rawDoc any, label string, q LogQuery) ([]string, error) {
	ctx = core.WithRequestMemo(core.WithAuditVerb(ctx, AuditVerbLogLabelValues))
	if err := checkDiscoverableLabel(label); err != nil {
		return nil, err
	}
	v, err := newProjectView(project, rawDoc, &q)
	if err != nil {
		return nil, err
	}
	if err := q.validate(); err != nil {
		return nil, err
	}
	_, resources, err := s.resolveProjectView(ctx, v, nil)
	if err != nil {
		return nil, err
	}
	reads := make([][]string, len(resources))
	errs := make([]error, len(resources))
	var g errgroup.Group
	g.SetLimit(maxConcurrentLogReads)
	for i, r := range resources {
		if core.IsDatastoreResource(r.ID) && !slices.Contains(datastoreLogLabels, label) {
			continue
		}
		g.Go(func() error {
			reads[i], errs[i] = s.LogLabelValues(ctx, label, q.forResource(r.ID))
			return nil
		})
	}
	_ = g.Wait()
	values := []string{}
	for i, err := range errs {
		switch {
		case err == nil:
			values = append(values, reads[i]...)
		case errors.Is(err, core.ErrForbidden) || errors.Is(err, core.ErrNotFound):
		default:
			return nil, err
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(values))), nil
}

func resourceIDs(rs []savedsearch.Resource) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

func (s *Service) projectRescopeInterval() time.Duration {
	if s.ProjectRescopeInterval > 0 {
		return s.ProjectRescopeInterval
	}
	return DefaultProjectRescopeInterval
}

// projectFollower is one resource's tail inside a project tail; its identity
// tells a stopped follower's late result from its successor's.
type projectFollower struct {
	id     string
	cancel context.CancelFunc
}

type projectFollowEnd struct {
	f   *projectFollower
	err error
}

// followProject tails a project view: one follower per resource in scope,
// merged onto one stream, with the scope re-resolved every
// projectRescopeInterval so a resource that joins starts streaming and one
// that leaves stops. A follower whose resource refuses or is gone ends
// quietly (the next resolution decides whether it is back); any other
// follower error, and a resolution that fails (the project gone, access
// revoked, the scope past the cap), ends the whole tail with that error.
func (s *Service) followProject(ctx context.Context, v projectView, q LogQuery, initial []savedsearch.Resource, emit func(LogEntry) error, heartbeat func() error) error {
	ctx = core.WithAuditVerb(ctx, AuditVerbFollowLogs)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	followerEmit, followerBeat := serializedWriters(emit, heartbeat)
	done := make(chan projectFollowEnd)
	active := map[string]*projectFollower{}
	running := 0
	start := func(id string) {
		fctx, fcancel := context.WithCancel(ctx)
		f := &projectFollower{id: id, cancel: fcancel}
		active[id] = f
		running++
		go func() {
			done <- projectFollowEnd{f: f, err: s.followLogs(fctx, q.forResource(id), followerEmit, followerBeat)}
		}()
	}
	stopAll := func() {
		cancel()
		for ; running > 0; running-- {
			<-done
		}
	}
	defer stopAll()
	for _, r := range initial {
		start(r.ID)
	}
	rescope := time.NewTicker(s.projectRescopeInterval())
	defer rescope.Stop()
	// A scope with no resource has no follower to keep the stream alive.
	beat := time.NewTicker(s.tailHeartbeatInterval())
	defer beat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case end := <-done:
			running--
			end.f.cancel()
			if ctx.Err() != nil {
				return nil // the tail itself ended; its followers only followed
			}
			if active[end.f.id] != end.f {
				continue // stopped by a rescope; its successor, if any, runs on
			}
			delete(active, end.f.id)
			if end.err == nil || errors.Is(end.err, context.Canceled) || errors.Is(end.err, core.ErrForbidden) || errors.Is(end.err, core.ErrNotFound) {
				continue
			}
			return end.err
		case <-rescope.C:
			followed := make(map[string]bool, len(active))
			for id := range active {
				followed[id] = true
			}
			_, next, err := s.resolveProjectView(ctx, v, followed)
			if err != nil {
				return err
			}
			want := map[string]bool{}
			for _, r := range next {
				want[r.ID] = true
				if active[r.ID] == nil {
					if _, err := q.forResource(r.ID).liveSubscribable(); err == nil {
						start(r.ID)
					}
				}
			}
			for id, f := range active {
				if !want[id] {
					f.cancel()
					delete(active, id)
				}
			}
		case <-beat.C:
			if len(active) == 0 {
				if err := followerBeat(); err != nil {
					return err
				}
			}
		}
	}
}
