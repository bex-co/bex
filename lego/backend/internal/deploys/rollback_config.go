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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// EnvironmentRestorer makes a service's saved env vars and secret files equal a
// rollback target's — secrets.Service.RestoreEnvironment. An interface so this
// package does not depend on the secrets package; nil keeps rollback image-only,
// which is also what every surface did before w1/m152.
type EnvironmentRestorer interface {
	RestoreEnvironment(ctx context.Context, service string, env, files map[string]string) (bool, error)
}

// RollbackOptions carries what differs between rollback surfaces.
type RollbackOptions struct {
	// DisableAutoDeploy turns spec.autoDeploy off in the same patch. Render does
	// this for a rollback triggered from its dashboard — "so the next push doesn't
	// silently undo the rollback" — and explicitly NOT for one triggered through its
	// API (render.com/docs/rollbacks). So the dashboard sets it; REST and MCP do not.
	DisableAutoDeploy bool
}

// targetConfig is what a rollback restores besides the image.
type targetConfig struct {
	startCommand string
}

// restoreTargetConfig makes the saved configuration equal the rollback target's
// (w1/m152 t009). Render: environment variables and start command "match the
// target deploy"; plan, custom domains and disks do not change.
//
// The source is what the operator recorded when that release dispatched: the
// release record `<name>-podtemplate-r<gen>` (its restorable spec) plus the
// `<source>-r<gen>` snapshots of the service's own env and files Secrets. The
// record's presence marks a release that was snapshotted and not yet reclaimed, so
// with it present a missing env or files snapshot means the target had none, and
// the restore empties that map.
//
// Returns (nil, nil) when the target has no record — it predates snapshots, or it
// is older than the 20-generation window GC keeps. The rollback then restores the
// image only, as it always did; that is logged rather than failed, because an
// image-only rollback is still what the user can usefully get. Linked env groups
// are shared with other services and are deliberately not rewritten by one
// service's rollback.
func (s *Service) restoreTargetConfig(ctx context.Context, a *appv1alpha1.App, target store.Deploy) (*targetConfig, error) {
	if s.Environment == nil || target.Generation <= 0 {
		return nil, nil
	}
	rec := &corev1.Secret{}
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: appv1alpha1.ReleaseRecordName(a.Name, target.Generation)}, rec)
	if apierrors.IsNotFound(err) {
		logf.FromContext(ctx).Info("rollback restores the image only: the target release has no configuration record",
			"app", a.Name, "deploy", target.ID, "generation", target.Generation)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var spec appv1alpha1.ReleaseRecordSpec
	if raw := rec.Data[appv1alpha1.ReleaseRecordSpecKey]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &spec); err != nil {
			return nil, fmt.Errorf("decode release record for generation %d: %w", target.Generation, err)
		}
	}
	envSource := a.Spec.EnvFromSecret
	if envSource == "" {
		envSource = a.Name + "-env"
	}
	env, err := s.snapshotData(ctx, a.Namespace, appv1alpha1.ReleaseSnapshotName(envSource, target.Generation))
	if err != nil {
		return nil, err
	}
	files, err := s.snapshotData(ctx, a.Namespace, appv1alpha1.ReleaseSnapshotName(a.Name+"-files", target.Generation))
	if err != nil {
		return nil, err
	}
	if _, err := s.Environment.RestoreEnvironment(ctx, a.Name, env, files); err != nil {
		return nil, fmt.Errorf("restore the target's environment: %w", err)
	}
	return &targetConfig{startCommand: spec.StartCommand}, nil
}

// snapshotData reads a snapshot's keys as strings; a missing snapshot is an empty
// map (see restoreTargetConfig on why that means "the target had none").
func (s *Service) snapshotData(ctx context.Context, namespace, name string) (map[string]string, error) {
	sec := &corev1.Secret{}
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, sec)
	if apierrors.IsNotFound(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(sec.Data))
	for k, v := range sec.Data {
		out[k] = string(v)
	}
	return out, nil
}
