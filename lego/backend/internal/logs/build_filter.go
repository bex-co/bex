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
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// LabelBuild is the build a build line belongs to: Render's `build` log label
// and filter, added to its contract on 2026-11-16 (w8/m442, ADR018 round 8).
// bex's build is its deploy's own, so a service build's ID is its deploy's
// `dep-` id: the id pipeline_minutes_exhausted's buildId carries (w9/m345) and
// the instance a deploy's platform lines carry. Render's own value for a
// service build is not observed yet. bex's sandbox image builds write no log
// lines, so no line carries an `sbb-` build.
const LabelBuild = "build"

// buildIndex names each build Job of an App by the deploy that ran it, so a
// build pod's line can carry its build and a build filter can select the Jobs
// of the builds it names. A build Job is named for its release generation
// (appv1alpha1.BuildJobName), as scopeToDeploy reads it.
type buildIndex map[string]string // build Job name → deploy id

// newBuildIndex indexes rows (newest first). A generation two rows share goes
// to the row that built: the other ran no build of its own.
func newBuildIndex(app string, rows []DeployProgress) buildIndex {
	index := buildIndex{}
	for _, built := range []bool{true, false} {
		for _, d := range rows {
			if d.Built != built || d.Generation <= 0 {
				continue
			}
			job := appv1alpha1.BuildJobName(app, appv1alpha1.BuildRevision(d.Generation))
			if _, taken := index[job]; !taken {
				index[job] = d.ID
			}
		}
	}
	return index
}

// buildOf is the build a pod's lines belong to, or "" for a pod no build Job
// owns (a kpack build, a pre-deploy step).
func (index buildIndex) buildOf(pod string) string {
	// A Job's pod suffix has no dash (isJobPod), so the Job is all before the last.
	if i := strings.LastIndexByte(pod, '-'); i > 0 && i < len(pod)-1 {
		return index[pod[:i]]
	}
	return ""
}

// jobsOf is the build Jobs of the given builds, sorted.
func (index buildIndex) jobsOf(builds []string) []string {
	var jobs []string
	for job, deploy := range index {
		if slices.Contains(builds, deploy) {
			jobs = append(jobs, job)
		}
	}
	slices.Sort(jobs)
	return jobs
}

// label gives each build pod line its build. A platform line already carries
// its deploy's (progressEntry).
func (index buildIndex) label(entries []LogEntry) []LogEntry {
	if len(index) == 0 {
		return entries
	}
	for _, e := range entries {
		if e.Labels[LabelType] != LogTypeBuild || e.Labels[LabelBuild] != "" {
			continue
		}
		if build := index.buildOf(e.Labels[LabelInstance]); build != "" {
			e.Labels[LabelBuild] = build
		}
	}
	return entries
}

// isJobPod reports whether pod is one of job's pods: the Job's name plus the
// controller's one dash-free random suffix, so `bld-x-gen-1` never takes
// `bld-x-gen-12`'s pod.
func isJobPod(pod, job string) bool {
	rest, ok := strings.CutPrefix(pod, job+"-")
	return ok && rest != "" && !strings.Contains(rest, "-")
}

// jobPodPattern is the store's pod-label pattern for the pods of jobs, the
// same match isJobPod makes.
func jobPodPattern(jobs []string) string {
	alts := make([]string, 0, len(jobs))
	for _, job := range jobs {
		alts = append(alts, regexp.QuoteMeta(job))
	}
	return "^(" + strings.Join(alts, "|") + ")-[^-]+$"
}

// narrowToBuilds points a build-filtered query at build lines, the only lines
// that carry a build, and reports false when the query asks for no build lines,
// which a build filter then leaves nothing of. A query without one is
// unchanged.
func (q *LogQuery) narrowToBuilds() bool {
	if len(q.Build) == 0 {
		return true
	}
	if !q.wants(LogTypeBuild) {
		return false
	}
	q.Types = []string{LogTypeBuild}
	return true
}

// inBuilds reports whether a build pod, or a deploy's platform lines named by
// its id, belongs to a build the query's build filter names. Without a build
// filter everything does.
func (q LogQuery) inBuilds(name string) bool {
	if len(q.Build) == 0 {
		return true
	}
	return slices.Contains(q.Build, name) ||
		slices.ContainsFunc(q.includeJobs, func(job string) bool { return isJobPod(name, job) })
}

// includeJobsMatcher selects the pods of the Jobs a build filter names. A
// filter that names no Job this App ran (an unknown build, or one that ran
// none) selects no pod at all, never every pod.
func includeJobsMatcher(q LogQuery) string {
	if len(q.Build) == 0 {
		return ""
	}
	if len(q.includeJobs) == 0 {
		return fmt.Sprintf("%s=%q", lokiLabelFor(LabelInstance), noSuchInstance)
	}
	return fmt.Sprintf("%s=~%q", lokiLabelFor(LabelInstance), jobPodPattern(q.includeJobs))
}

// scopeToBuilds is an App read's build attribution: the index that labels its
// build lines, and q's build filter resolved to the Jobs of the builds it
// names (q.includeJobs). Only a read of build lines needs either. Labels are
// best-effort, so a failed read of the deploy rows labels nothing; a filter
// cannot resolve without them, and answers the error rather than another
// build's lines or an empty page that would read as "no such build".
func (s *Service) scopeToBuilds(ctx context.Context, resource string, q *LogQuery) (buildIndex, error) {
	if !slices.Contains(q.Types, LogTypeBuild) {
		return nil, nil
	}
	index, err := s.buildIndexOf(ctx, resource, q.App)
	if err != nil {
		if len(q.Build) > 0 {
			return nil, fmt.Errorf("resolve the build filter: %w", err)
		}
		return nil, nil
	}
	if len(q.Build) > 0 {
		q.includeJobs = index.jobsOf(q.Build)
	}
	return index, nil
}

// buildIndexOf indexes the build Jobs of app (its CR name) from the deploy
// rows of resource (its public id). nil without a deploy source.
func (s *Service) buildIndexOf(ctx context.Context, resource, app string) (buildIndex, error) {
	if s.DeployProgress == nil {
		return nil, nil
	}
	rows, err := s.DeployProgress(ctx, resource, time.Time{})
	if err != nil {
		return nil, err
	}
	return newBuildIndex(app, rows), nil
}
