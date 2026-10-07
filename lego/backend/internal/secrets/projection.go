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
	"time"

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

// sourceProjection is what projectSource found: whether a's own Secret existed
// (replacing a predecessor's counts as creating), whether a newer revision
// already owned it, so this projection was left to that write, and whether the
// Secret as the call left it holds any key.
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
//
// Two Secrets are exceptions to the order. One is a Secret another App
// controls: a deleted namesake's, which garbage collection has not reached.
// Its revision belongs to a store the purge removed, so a's write replaces it
// whatever it records, provided a is still the App at that name (w5/118). The
// other is a preparation its create abandoned, ownerless long past any
// create's window: no store's write ever stamped it after the preparation's,
// so a's write replaces it too (w5/148).
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
		// A Secret another App controls is a predecessor's that garbage
		// collection has not reached yet. A service recreated under the same
		// name restarts its store at version 1, so the old revision must not
		// outrank it: the Secret is replaced, and owned by a (w5/118).
		// An abandoned preparation is replaced too, but counts as a's own
		// Secret when it existed: it may be a's, if a's adoption crashed, and a
		// failed write must restore it rather than remove it (w5/148).
		owner := metav1.GetControllerOfNoCopy(sec)
		replaced := owner != nil && owner.UID != a.UID
		predecessor := replaced || abandonedPreparation(sec, kind, s.Now())
		// Any Secret a does not control yet, a predecessor's or one a create
		// prepared ownerless, becomes a's only while a is still the App at its
		// name. A write in flight for a deleted App would otherwise take over its
		// namesake's prepared Secret (w5/147).
		if !metav1.IsControlledBy(sec, a) {
			if err := s.confirmLiveApp(ctx, a); err != nil {
				return out, err
			}
		}
		out.ExistedBefore = !replaced
		project := data
		if current, known := projectedRevision(sec, kind); known && !predecessor {
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

// stampProjection sets sec to data at revision, owned by a. A predecessor's
// controller reference names the same App, so SetControllerReference, which
// matches owners by group, kind and name, not UID, replaces it with a's.
func (s *Service) stampProjection(a *appv1alpha1.App, sec *corev1.Secret, kind projectionKind, data map[string]string, revision sourceRevision) error {
	setProjectedRevision(sec, kind, revision)
	sec.Type = corev1.SecretTypeOpaque
	sec.Data = envBytes(data)
	return controllerutil.SetControllerReference(a, sec, s.Client.Scheme())
}

// vacated reports whether a's create may take sec, the Secret at one of its
// projection names, over, because no App holds that name any more. Either sec
// is a deleted service's Secret that garbage collection has not reached, its
// controller the App that held the name, or it is a preparation its create
// abandoned: ownerless long past any create's window, as a bex-api crash
// between prepare and commit leaves one (w5/135). A live App at the name, any
// other controller, or a preparation that may still be running holds it.
func (s *Service) vacated(ctx context.Context, a *appv1alpha1.App, kind projectionKind, sec *corev1.Secret) (bool, error) {
	if owner := metav1.GetControllerOfNoCopy(sec); owner == nil {
		if !abandonedPreparation(sec, kind, s.Now()) {
			return false, nil
		}
	} else if owner.APIVersion != appv1alpha1.SchemeGroupVersion.String() || owner.Kind != "App" || owner.Name != a.Name {
		return false, nil
	}
	err := s.Client.Get(ctx, client.ObjectKeyFromObject(a), &appv1alpha1.App{})
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	return false, err
}

// preparedAtAnnotation dates a create's claim on a projection Secret
// (prepareProjection). A taken-over Secret keeps its first creationTimestamp,
// so the claim records its own time; adoption removes it.
const preparedAtAnnotation = "app.bex.co/prepared-at"

// preparationWindow is far past how long any create holds its prepared Secret
// ownerless: core.CreateSecretsTimeout, plus its 30s compensation.
const preparationWindow = 6 * core.CreateSecretsTimeout

func setPreparedAt(sec *corev1.Secret, at time.Time) {
	metav1.SetMetaDataAnnotation(&sec.ObjectMeta, preparedAtAnnotation, at.UTC().Format(time.RFC3339))
}

// abandonedPreparation reports whether sec, at kind's name, is a preparation
// left ownerless longer than any create runs. One from before claims were
// dated is dated by its creation, but only if it carries kind's projection
// revision: other ownerless Secrets (an env group's) are never a create's. One
// of unknown age, or with any owner, is never abandoned.
func abandonedPreparation(sec *corev1.Secret, kind projectionKind, now time.Time) bool {
	if len(sec.OwnerReferences) != 0 {
		return false
	}
	claimed, err := time.Parse(time.RFC3339, sec.Annotations[preparedAtAnnotation])
	if err != nil {
		if _, projected := projectedRevision(sec, kind); !projected {
			return false
		}
		claimed = sec.CreationTimestamp.Time
	}
	return !claimed.IsZero() && now.Sub(claimed) > preparationWindow
}

// confirmLiveApp refuses a write whose App is no longer the one at its name. A
// request that spanned a delete and a recreate under the same name would
// otherwise take the new service's Secret over as its predecessor's.
func (s *Service) confirmLiveApp(ctx context.Context, a *appv1alpha1.App) error {
	live := &appv1alpha1.App{}
	err := s.Client.Get(ctx, client.ObjectKeyFromObject(a), live)
	if apierrors.IsNotFound(err) || err == nil && live.UID != a.UID {
		return core.ErrServiceReplaced
	}
	return err
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
