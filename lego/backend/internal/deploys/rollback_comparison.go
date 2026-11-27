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
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rollback_comparison.go is Render's rollbackComparison read (w1/m646,
// docs/render-artifacts/rollback-comparison.md): the configuration the live
// release runs beside the one a rollback to a deploy would run, so the
// dashboard's Confirm Rollback page can list what changes before anyone
// confirms. Commands, the health check path and the instance count are
// values; env vars and secret files are compared here and leave as names only.

// RollbackComparison is the answer: the live deploy and the target, each with
// the configuration its release record holds (nil when it holds none), and
// whether the service's saved plan is smaller than the one the target ran on.
type RollbackComparison struct {
	// CurrentLiveDeploy is nil when no release serves.
	CurrentLiveDeploy *ComparedDeploy
	Deploy            ComparedDeploy
	// CurrentPlanLower is Render's plan warning: the saved plan, which a
	// rollback keeps, sits below the target's in the compute ladder.
	CurrentPlanLower bool
}

// ComparedDeploy is one side of the comparison.
type ComparedDeploy struct {
	ID string
	// Snapshot is nil when the release's record is missing (reclaimed past the
	// retained window, or older than the records) or predates record version
	// 1. Render shows "Unable to load configuration…" then, as bex does. A
	// static site's side is the environment its published files were built
	// with (staticBuildGeneration).
	Snapshot *ServiceConfigSnapshot
}

// ServiceConfigSnapshot is Render's snapshotFields. Plan is the compute plan
// the release's pod ran with, in Render's spelling ("" unknown).
type ServiceConfigSnapshot struct {
	RunCommand           string
	PreDeployCommand     string
	DockerCommand        string
	HealthCheckPath      string
	Instances            *int32
	Plan                 string
	EnvGroups            []SnapshotEnvGroup
	DeletedEnvGroupCount int
	// EnvVarChanges is empty on the live side. On the target it names what
	// changed after this deploy, as Render's sentences read it: a created key
	// is one the live release has and the target does not.
	EnvVarChanges EnvVarChanges
}

// SnapshotEnvGroup is a linked env group that still exists.
type SnapshotEnvGroup struct {
	ID   string
	Name string
}

// EnvVarChanges are the service's own env var keys and secret file names that
// changed after the target deploy, each list sorted and never nil. Values
// never appear.
type EnvVarChanges struct {
	CreatedVarKeys  []string
	DeletedVarKeys  []string
	UpdatedVarKeys  []string
	CreatedFileKeys []string
	DeletedFileKeys []string
	UpdatedFileKeys []string
}

// recordedRelease is one release's configuration as its record and copies
// hold it, values included. It never leaves this file: only snapshot and the
// key sets changesSince derives from env and files do.
type recordedRelease struct {
	snapshot ServiceConfigSnapshot
	// tier is the compute tier id Plan spells.
	tier  string
	env   map[string]string
	files map[string]string
}

// RollbackComparison compares the release serving now with deployID's. It
// reads like the deploy it compares (can_view_activity) and, since it names
// env var keys and secret files, with the access the service's Environment
// page reads them with (EnvironmentKeys: can_view_sensitive, which only roles
// that can also roll back hold).
func (s *Service) RollbackComparison(ctx context.Context, service, deployID string) (RollbackComparison, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanViewActivity, service)
	if err != nil {
		return RollbackComparison{}, err
	}
	if s.EnvironmentKeys != nil {
		if err := s.EnvironmentKeys(ctx, service); err != nil {
			return RollbackComparison{}, err
		}
	}
	if s.Store == nil {
		return RollbackComparison{}, core.ErrDeploysUnavailable
	}
	appID := appStoreID(a)
	if appID == "" {
		return RollbackComparison{}, core.NotFound("deploy")
	}
	target, err := s.Store.GetDeploy(ctx, appID, deployID)
	if errors.Is(err, store.ErrNotFound) {
		return RollbackComparison{}, core.NotFound("deploy")
	}
	if err != nil {
		return RollbackComparison{}, err
	}
	out := RollbackComparison{Deploy: ComparedDeploy{ID: target.ID}}
	// A serving release no deploy row records yet (core.ErrConflict) has no
	// comparable side: the dashboard says it could not load the live deploy's.
	live, err := s.servingDeploy(ctx, service, a)
	if err != nil && !errors.Is(err, core.ErrConflict) {
		return RollbackComparison{}, err
	}
	if live.ID == "" {
		return out, nil
	}
	out.CurrentLiveDeploy = &ComparedDeploy{ID: live.ID}
	groups, err := s.workspaceGroupNames(ctx, a)
	if err != nil {
		return RollbackComparison{}, err
	}
	liveGeneration, targetGeneration := live.Generation, target.Generation
	if a.Spec.Type == appv1alpha1.TypeStaticSite {
		if liveGeneration, err = s.staticBuildGeneration(ctx, a, liveGeneration); err != nil {
			return RollbackComparison{}, err
		}
		if targetGeneration, err = s.staticBuildGeneration(ctx, a, targetGeneration); err != nil {
			return RollbackComparison{}, err
		}
	}
	// The live side shows the count live runs: the record's, unless live is
	// itself a selected release (a rollback or restart serving), which the
	// operator renders through the selection guard below. The target side
	// always shows what a rollback will run: a rollback materializes through
	// that guard, and its future generation can match no scale stamp on the
	// App today.
	liveSelected := false
	if ref := a.ActiveReleaseConfig(); ref != nil && ref.SourceGeneration > 0 && ref.Generation == live.Generation {
		liveSelected = true
	}
	current, err := s.recordedRelease(ctx, a, liveGeneration, groups, previewInstances{selected: liveSelected, scaledRelease: live.Generation})
	if err != nil {
		return RollbackComparison{}, err
	}
	old, err := s.recordedRelease(ctx, a, targetGeneration, groups, previewInstances{selected: true})
	if err != nil {
		return RollbackComparison{}, err
	}
	if current != nil {
		out.CurrentLiveDeploy.Snapshot = &current.snapshot
	}
	if old != nil {
		if current != nil {
			old.snapshot.EnvVarChanges = changesSince(old, current)
		}
		out.Deploy.Snapshot = &old.snapshot
		out.CurrentPlanLower = planBelow(a.Spec.Tier, old.tier)
	}
	return out, nil
}

// staticBuildGenerationHops bounds staticBuildGeneration's walk back through
// copies of copies; the retained window is no deeper.
const staticBuildGenerationHops = 20

// staticBuildGeneration is the release whose build a static site's release at
// generation serves: generation itself, unless that release is a Rollback or
// Restart, which copied an earlier release's retained publication (w4/m197,
// w32/049) and so serves files that release's build produced, with the
// environment that build read. Its own record names the sources saved when
// it was requested, which its files never used. The copy is the StaticPrefix
// of the release selection its record's adopted spec holds, followed through
// earlier copies. 0, which compares as unavailable, when a copy names no
// release or the walk does not end within the retained window.
func (s *Service) staticBuildGeneration(ctx context.Context, a *appv1alpha1.App, generation int64) (int64, error) {
	for range staticBuildGenerationHops {
		if generation <= 0 {
			return 0, nil
		}
		rec, err := s.secretData(ctx, a, appv1alpha1.ReleaseRecordName(a.Name, generation))
		if rec == nil || err != nil {
			return generation, err
		}
		var adopted appv1alpha1.AppSpec
		if raw := rec[appv1alpha1.ReleaseRecordAdoptedSpecKey]; len(raw) == 0 || json.Unmarshal(raw, &adopted) != nil {
			return generation, nil
		}
		ref := adopted.ReleaseConfig
		if ref == nil || ref.Generation != generation || ref.StaticPrefix == "" {
			return generation, nil
		}
		copied, ok := appv1alpha1.StaticPrefixGeneration(ref.StaticPrefix)
		if !ok || copied >= generation {
			return 0, nil
		}
		generation = copied
	}
	return 0, nil
}

// workspaceGroupNames is a's workspace's env groups, id to name; nil when no
// group reader is wired, which counts every recorded group as surviving.
func (s *Service) workspaceGroupNames(ctx context.Context, a *appv1alpha1.App) (map[string]string, error) {
	if s.EnvGroupIDsByName == nil {
		return nil, nil
	}
	byName, err := s.EnvGroupIDsByName(core.WithWorkspace(ctx, a.Labels[core.LabelTenant]))
	if err != nil {
		return nil, fmt.Errorf("read the workspace's environment groups: %w", err)
	}
	names := make(map[string]string, len(byName))
	for name, gid := range byName {
		names[gid] = name
	}
	return names, nil
}

// previewInstances tells recordedSnapshot which instance count a release
// from the record runs under the App's current saved state. selected is
// whether this side materializes through the operator's selection guard
// (projectRuntimeApp): a rollback always does; the live side only when the
// live release is itself a selection. scaledRelease is the release
// generation an explicit scale must stamp to supersede the record; 0
// disables the arm.
type previewInstances struct {
	selected      bool
	scaledRelease int64
}

// recordedRelease reads generation's record and the service's own env and
// file copies. nil, with no error, when any of them is gone or the record
// predates version 1: half a configuration would list changes that are not.
func (s *Service) recordedRelease(ctx context.Context, a *appv1alpha1.App, generation int64, groups map[string]string, preview previewInstances) (*recordedRelease, error) {
	if generation <= 0 {
		return nil, nil
	}
	rec, err := s.secretData(ctx, a, appv1alpha1.ReleaseRecordName(a.Name, generation))
	if rec == nil || err != nil {
		return nil, err
	}
	var spec appv1alpha1.ReleaseRecordSpec
	if raw := rec[appv1alpha1.ReleaseRecordSpecKey]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &spec); err != nil {
			return nil, fmt.Errorf("decode release record for generation %d: %w", generation, err)
		}
	}
	if spec.Version < 1 {
		return nil, nil
	}
	r := &recordedRelease{snapshot: recordedSnapshot(a, spec, preview), env: map[string]string{}, files: map[string]string{}}
	// A static site runs no instances, so it has no plan to compare.
	if a.Spec.Type != appv1alpha1.TypeStaticSite {
		r.tier = recordedTier(rec[appv1alpha1.ReleaseRecordPodTemplateKey])
	}
	if t, ok := tiers.Compute.ByID(r.tier); ok {
		r.snapshot.Plan = t.RenderPlan
	}
	if spec.EnvFromSecret != "" {
		data, err := s.secretData(ctx, a, appv1alpha1.AppReleaseSnapshotName(a.Name, spec.EnvFromSecret, generation))
		if data == nil || err != nil {
			return nil, err
		}
		for k, v := range data {
			r.env[k] = "secret:" + string(v)
		}
	}
	// Literal entries (a Blueprint's fromDatabase and fromService references)
	// override the env Secret's keys in the pod, as Kubernetes orders them.
	for _, e := range spec.Env {
		raw, _ := json.Marshal(e)
		r.env[e.Name] = "literal:" + string(raw)
	}
	var groupIDs []string
	for _, ref := range spec.EnvFromSecrets {
		if gid, ok := rollout.GroupID(ref); ok && !slices.Contains(groupIDs, gid) {
			groupIDs = append(groupIDs, gid)
		}
	}
	for _, ref := range spec.FilesFromSecrets {
		if gid, ok := rollout.GroupID(ref); ok {
			if !slices.Contains(groupIDs, gid) {
				groupIDs = append(groupIDs, gid)
			}
			continue
		}
		data, err := s.secretData(ctx, a, appv1alpha1.AppReleaseSnapshotName(a.Name, ref, generation))
		if data == nil || err != nil {
			return nil, err
		}
		for k, v := range data {
			r.files[k] = string(v)
		}
	}
	for _, gid := range groupIDs {
		name, ok := groups[gid]
		switch {
		case groups == nil:
			r.snapshot.EnvGroups = append(r.snapshot.EnvGroups, SnapshotEnvGroup{ID: gid, Name: gid})
		case ok:
			r.snapshot.EnvGroups = append(r.snapshot.EnvGroups, SnapshotEnvGroup{ID: gid, Name: name})
		default:
			r.snapshot.DeletedEnvGroupCount++
		}
	}
	return r, nil
}

// secretData is a Secret's data in a's namespace; nil, with no error, when it
// does not exist.
func (s *Service) secretData(ctx context.Context, a *appv1alpha1.App, name string) (map[string][]byte, error) {
	sec := &corev1.Secret{}
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: name}, sec)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sec.Data == nil {
		return map[string][]byte{}, nil
	}
	return sec.Data, nil
}

// recordedSnapshot is the record's commands, health check path and instance
// count, in the fields Render's snapshot names, for the rows that apply to a's
// type: a static site has none of them, only the environment its build read;
// a cron job runs a command and has no instances, health check or pre-deploy
// step; and only a web service has an HTTP health check path.
// The instance count is the one the release runs under the App's current
// saved state (previewInstanceCount), so the page never lists a change the
// rollback will not make.
func recordedSnapshot(a *appv1alpha1.App, spec appv1alpha1.ReleaseRecordSpec, preview previewInstances) ServiceConfigSnapshot {
	none := []string{}
	snap := ServiceConfigSnapshot{EnvVarChanges: EnvVarChanges{none, none, none, none, none, none}}
	if a.Spec.Type == appv1alpha1.TypeStaticSite {
		return snap
	}
	command := spec.StartCommand
	cron := a.Spec.Type == appv1alpha1.TypeCronJob
	if cron && spec.Command != nil && *spec.Command != "" {
		command = *spec.Command
	}
	// The start command is Render's Docker Command for a Dockerfile build and
	// a prebuilt image, as REST's envSpecificDetails names it.
	if a.Spec.Runtime == "docker" || a.Spec.Runtime == "image" || (a.Spec.Runtime == "" && a.Spec.Image != "") {
		snap.DockerCommand = command
	} else {
		snap.RunCommand = command
	}
	if cron {
		return snap
	}
	// A record from before w4/m433 has no pre-deploy command: a rollback to it
	// runs the saved one.
	snap.PreDeployCommand = a.Spec.PreDeployCommand
	if spec.Runtime != nil {
		snap.PreDeployCommand = spec.Runtime.PreDeployCommand
	}
	if (a.Spec.Type == "" || a.Spec.Type == appv1alpha1.TypeWebService) && spec.HealthCheckPath != nil {
		snap.HealthCheckPath = *spec.HealthCheckPath
	}
	if spec.Replicas != nil {
		snap.Instances = new(previewInstanceCount(a, spec, preview))
	}
	return snap
}

// previewInstanceCount is the instance count a release from spec's record
// runs under a's current saved state. A normal (unselected) live deploy runs
// the App's current saved count directly when Scale() moved it past the
// record's SavedReplicas; a selected release (rollback/restart) mirrors the
// operator's selection guard (projectRuntimeApp in release_config_selection.go):
// the record's count runs unless an explicit scale stamped this release, or the
// saved count moved on since the record was written — then the saved count wins.
// Legacy records that predate SavedReplicas keep the record's count. Zero is
// the default: one instance (the operator's effectiveReplicas).
func previewInstanceCount(a *appv1alpha1.App, spec appv1alpha1.ReleaseRecordSpec, preview previewInstances) int32 {
	count := int32(1)
	if spec.Replicas != nil {
		count = max(*spec.Replicas, 1)
	}
	if preview.selected && preview.scaledRelease > 0 &&
		a.Annotations[appv1alpha1.AnnotationReleaseConfigScaled] == strconv.FormatInt(preview.scaledRelease, 10) {
		return max(a.Spec.Replicas, 1)
	}
	if spec.SavedReplicas != nil && *spec.SavedReplicas != a.Spec.Replicas {
		return max(a.Spec.Replicas, 1)
	}
	return count
}

// recordedTier is the compute tier whose CPU and memory the recorded pod
// template's app container ran with; "" when none matches.
func recordedTier(raw []byte) string {
	var tmpl corev1.PodTemplateSpec
	if len(raw) == 0 || json.Unmarshal(raw, &tmpl) != nil {
		return ""
	}
	for _, c := range tmpl.Spec.Containers {
		if c.Name != appv1alpha1.ReleaseRecordAppContainer {
			continue
		}
		cpu, mem := c.Resources.Limits[corev1.ResourceCPU], c.Resources.Limits[corev1.ResourceMemory]
		for _, id := range tiers.Compute.IDs() {
			tc, tm, _, _ := tiers.Compute.Resources(id)
			if cpu.Cmp(resource.MustParse(tc)) == 0 && mem.Cmp(resource.MustParse(tm)) == 0 {
				return id
			}
		}
	}
	return ""
}

// planBelow reports whether the saved tier sits below target's in the compute
// ladder, cheapest first, as Render compares the plans' prices. An unknown
// target tier warns of nothing.
func planBelow(saved, target string) bool {
	ladder := tiers.Compute.IDs()
	t := slices.Index(ladder, target)
	if t < 0 {
		return false
	}
	if saved == "" {
		saved = tiers.Compute.Default().ID
	}
	return slices.Index(ladder, saved) < t
}

// changesSince names the service's own env vars and secret files that changed
// after old's release, by comparing it with current's: a created key is one
// current has and old does not.
func changesSince(old, current *recordedRelease) EnvVarChanges {
	var c EnvVarChanges
	c.CreatedVarKeys, c.DeletedVarKeys, c.UpdatedVarKeys = keyChanges(old.env, current.env)
	c.CreatedFileKeys, c.DeletedFileKeys, c.UpdatedFileKeys = keyChanges(old.files, current.files)
	return c
}

func keyChanges(old, current map[string]string) (created, deleted, updated []string) {
	created, deleted, updated = []string{}, []string{}, []string{}
	for _, k := range slices.Sorted(maps.Keys(current)) {
		v, ok := old[k]
		switch {
		case !ok:
			created = append(created, k)
		case v != current[k]:
			updated = append(updated, k)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(old)) {
		if _, ok := current[k]; !ok {
			deleted = append(deleted, k)
		}
	}
	return created, deleted, updated
}
