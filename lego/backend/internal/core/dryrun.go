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

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A dry-run is the real call stopped before its first write (w5/m116): the
// verb runs the same plan its real call does, ending in a server-side dry-run
// of the exact object it would write, and returns the preview instead.

type dryRunKey struct{}

// PreviewOf marks ctx as a dry-run of a verb the caller is authorized only to
// view, on the resource labels describe. A preview runs every check its real
// call runs, but billing gates only for a caller who may perform the call: a
// 402 discloses payment state that otherwise needs can_manage_billing, and the
// real call refuses anyone else before it gets that far. Whether the caller may
// — every one of relations, probed without an audit row on the object the real
// call authorizes against — is asked only when a billing gate is reached.
func (b *Base) PreviewOf(ctx context.Context, labels map[string]string, relations ...string) context.Context {
	mayPerform := sync.OnceValue(func() bool {
		object, err := b.resourceWorkspace(ctx, labels)
		if err != nil {
			return false
		}
		for _, relation := range relations {
			if !b.CanDecisionOn(ctx, relation, object).Allowed() {
				return false
			}
		}
		return true
	})
	return context.WithValue(ctx, dryRunKey{}, mayPerform)
}

// billingHiddenFromPreview reports a dry-run by a caller who may not perform
// the real verb (PreviewOf).
func billingHiddenFromPreview(ctx context.Context) bool {
	mayPerform, dryRun := ctx.Value(dryRunKey{}).(func() bool)
	return dryRun && !mayPerform()
}

// DryRunCreate runs obj's Create through the API server's admission — schema
// and CEL rules, admission policies, and the workspace ResourceQuota — without
// persisting anything, so a create preview meets exactly the refusals its real
// call would. obj itself is left untouched.
//
// A workspace's first resource lands in a namespace the real call creates
// first. Until then the object is validated in the bootstrap namespace, without
// its workspace label (the only form the app admission policy accepts there):
// nothing exists yet to count against the workspace's caps, and every plan's
// caps are at least one.
func (b *Base) DryRunCreate(ctx context.Context, obj client.Object) error {
	probe := obj.DeepCopyObject().(client.Object)
	err := b.Client.Create(ctx, probe, client.DryRunAll)
	if !apierrors.IsNotFound(err) || obj.GetNamespace() == b.Namespace {
		return err
	}
	probe = obj.DeepCopyObject().(client.Object)
	probe.SetNamespace(b.Namespace)
	labels := probe.GetLabels()
	delete(labels, LabelWorkspace)
	probe.SetLabels(labels)
	err = b.Client.Create(ctx, probe, client.DryRunAll)
	if apierrors.IsAlreadyExists(err) {
		// Admission validated the object before storage refused its name,
		// which is taken only in the stand-in namespace.
		return nil
	}
	return err
}

// PatchObject merge-patches obj through the API server's admission — only as a
// dry-run when dryRun is set — and answers an invalid field the way
// InvalidFieldsError does, so a write and its dry-run refuse alike.
func (b *Base) PatchObject(ctx context.Context, obj client.Object, patch client.Patch, dryRun bool) error {
	var opts []client.PatchOption
	if dryRun {
		opts = append(opts, client.DryRunAll)
	}
	err := b.Client.Patch(ctx, obj, patch, opts...)
	if mapped, ok := InvalidFieldsError(err); ok {
		return mapped
	}
	return err
}

// InvalidFieldsError maps the API server's refusal of an object that fails its
// schema or CEL rules onto a 400 naming each invalid field and why — the same
// answer for a real write and its dry-run. It carries only the causes that name
// a field, never the object's name, which a dry-run cannot share with its real
// call; the API server's fieldless note that rules went unchecked behind an
// earlier failure adds nothing the caller can act on. ok is false for any
// other error.
func InvalidFieldsError(err error) (mapped error, ok bool) {
	details, ok := InvalidDetails(err)
	if !ok {
		return nil, false
	}
	var causes []string
	for _, c := range details.Causes {
		if c.Field == "" || c.Field == "<nil>" {
			continue
		}
		causes = append(causes, c.Field+": "+c.Message)
	}
	if len(causes) == 0 {
		return fmt.Errorf("%w: the configuration is invalid", ErrBadRequest), true
	}
	return fmt.Errorf("%w: %s", ErrBadRequest, strings.Join(causes, "; ")), true
}

// InvalidDetails returns the API server's details of an Invalid refusal: the
// refused object's kind and name, and each cause. ok is false for any other
// error.
func InvalidDetails(err error) (details metav1.StatusDetails, ok bool) {
	if !apierrors.IsInvalid(err) {
		return metav1.StatusDetails{}, false
	}
	var status apierrors.APIStatus
	if errors.As(err, &status) && status.Status().Details != nil {
		details = *status.Status().Details
	}
	return details, true
}
