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

package secrets

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	// envProjectionRevisionAnnotation and filesProjectionRevisionAnnotation
	// record the store revision a service's env or secret-files Secret holds.
	envProjectionRevisionAnnotation   = "app.bex.co/env-source-revision"
	filesProjectionRevisionAnnotation = "app.bex.co/files-source-revision"
	// provisionalProjectionAnnotation marks a revision a delete stamped before
	// its commit created it.
	provisionalProjectionAnnotation = "app.bex.co/source-revision-provisional"
)

// errProjectionConflict is a projection that lost to a concurrent write. It
// names no object or revision and wraps no public error, so each caller
// chooses its own answer.
var errProjectionConflict = errors.New("the projection lost to a concurrent write")

// projectionKind is one derived Secret a service's store map projects into:
// its name and the annotation that records the store revision it holds.
type projectionKind struct {
	secretName func(app string) string
	annotation string
}

var (
	envProjection   = projectionKind{secretName: envSecretName, annotation: envProjectionRevisionAnnotation}
	filesProjection = projectionKind{secretName: filesSecretName, annotation: filesProjectionRevisionAnnotation}
)

// sourceRevision is the store revision a projection carries (w5/m127). A
// delete projects before it commits, so its revision is provisional: the one
// its commit will create, which another write may commit first.
type sourceRevision struct {
	version     uint64
	provisional bool
}

// committedAt is the revision of a map the store holds at version.
func committedAt(version uint64) sourceRevision { return sourceRevision{version: version} }

// provisionalAt is the revision a delete's commit will create at version.
func provisionalAt(version uint64) sourceRevision {
	return sourceRevision{version: version, provisional: true}
}

// after reports whether r orders after o: a higher version, or the same
// version committed where o is only provisional. The write that commits a
// version owns it, so its projection replaces a delete's provisional one, and
// the delete, whose commit then conflicts, re-reads.
func (r sourceRevision) after(o sourceRevision) bool {
	if r.version != o.version {
		return r.version > o.version
	}
	return o.provisional && !r.provisional
}

// sourceProjection is what projectSource found: whether the Secret existed,
// whether a newer revision already owned it, so this projection was left to
// that write, and whether the Secret as the call left it holds any key.
type sourceProjection struct {
	ExistedBefore bool
	Superseded    bool
	HoldsData     bool
}

// projectSource writes data into kind's Secret for a as the store revision it
// carries (w5/m127). An emptied map is written too: its Secret stays, empty, as
// the record of that revision, so a projection that arrives late loses to it
// rather than recreating the Secret (w5/106).
//
// A Secret a later revision already holds is left to that write: the result
// is Superseded, or errProjectionConflict for a delete's provisional revision,
// whose commit can then only conflict. The same committed revision with the
// same data is a no-op, and with other data a conflict, since one revision
// holds one map. Two deletes that read one revision project the same
// provisional one, and the Secret keeps only what both keep, so neither mounts
// the key the other revoked. An update carries the read object's
// resourceVersion, so a concurrent change is judged again rather than
// overwritten. A store without versions (test doubles) has no order to keep,
// and its writes are unconditional.
func (s *Service) projectSource(ctx context.Context, a *appv1alpha1.App, kind projectionKind, data map[string]string, revision sourceRevision) (sourceProjection, error) {
	name := kind.secretName(a.Name)
	if _, versioned := s.Store.(core.VersionedSecretKV); !versioned {
		return sourceProjection{HoldsData: len(data) > 0}, s.upsertSecret(ctx, a, name, data)
	}
	var out sourceProjection
	for attempt := 0; attempt <= casMaxRetries; attempt++ {
		sec := &corev1.Secret{}
		err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: name}, sec)
		if apierrors.IsNotFound(err) {
			out.ExistedBefore = false
			sec = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: a.Namespace}}
			if err := s.stampProjection(a, sec, kind, data, revision); err != nil {
				return out, err
			}
			if err := s.Client.Create(ctx, sec); err != nil {
				if apierrors.IsAlreadyExists(err) {
					continue // another write created it first: judge its revision
				}
				return out, err
			}
			out.HoldsData = len(data) > 0
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out.ExistedBefore = true
		project := data
		if current, known := projectedRevision(sec, kind); known {
			if current.after(revision) {
				out.Superseded, out.HoldsData = true, len(sec.Data) > 0
				if revision.provisional {
					return out, errProjectionConflict
				}
				return out, nil
			}
			if current == revision {
				if !equalSecretData(sec.Data, data) {
					if !revision.provisional {
						return out, errProjectionConflict
					}
					project = keptByBoth(sec.Data, data)
				}
				if equalSecretData(sec.Data, project) {
					out.HoldsData = len(project) > 0
					return out, nil
				}
			}
		}
		if err := s.stampProjection(a, sec, kind, project, revision); err != nil {
			return out, err
		}
		err = s.Client.Update(ctx, sec)
		if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
			continue // changed since the read: judge the new object
		}
		out.HoldsData = len(project) > 0
		return out, err
	}
	return out, errProjectionConflict
}

// keptByBoth is what two deletes that read one revision both keep: the
// entries of data that projected also holds.
func keptByBoth(projected map[string][]byte, data map[string]string) map[string]string {
	kept := make(map[string]string, len(data))
	for key, value := range data {
		if held, ok := projected[key]; ok && string(held) == value {
			kept[key] = value
		}
	}
	return kept
}

// stampProjection sets sec to data at revision, owned by a.
func (s *Service) stampProjection(a *appv1alpha1.App, sec *corev1.Secret, kind projectionKind, data map[string]string, revision sourceRevision) error {
	setProjectedRevision(sec, kind, revision)
	sec.Type = corev1.SecretTypeOpaque
	sec.Data = envBytes(data)
	return controllerutil.SetControllerReference(a, sec, s.Client.Scheme())
}

// setProjectedRevision records revision on sec for kind; projectedRevision
// reads it back.
func setProjectedRevision(sec *corev1.Secret, kind projectionKind, revision sourceRevision) {
	metav1.SetMetaDataAnnotation(&sec.ObjectMeta, kind.annotation, encodeEnvRevision(revision.version))
	if revision.provisional {
		metav1.SetMetaDataAnnotation(&sec.ObjectMeta, provisionalProjectionAnnotation, "true")
	} else {
		delete(sec.Annotations, provisionalProjectionAnnotation)
	}
}

// projectedRevision is the store revision sec records for kind. known is
// false for a Secret that records none this package can read, which any
// revision replaces.
func projectedRevision(sec *corev1.Secret, kind projectionKind) (revision sourceRevision, known bool) {
	version, err := decodeEnvRevision(sec.Annotations[kind.annotation])
	if err != nil {
		return sourceRevision{}, false
	}
	return sourceRevision{version: version, provisional: sec.Annotations[provisionalProjectionAnnotation] == "true"}, true
}
