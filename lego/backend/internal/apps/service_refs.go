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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// service_refs.go keeps Blueprint fromService port/hostport references true to
// the port their target actually serves (w6/m148, ADR089 § Port slice
// decisions). The apply resolves each one from the target's current serving
// port and records it here; ServiceRefPropagator rewrites the literal when that
// port changes or is first discovered.

// serviceRefsAnnotation records, on a consumer App, every port-carrying
// fromService reference its Blueprint declared: a sorted JSON list of
// serviceRefRecord. A host-only reference names the immutable slug, so it is
// never recorded.
const serviceRefsAnnotation = "app.bex.co/service-refs"

// serviceRefsLabel marks the Apps that carry serviceRefsAnnotation, so the
// propagator lists only them (bex-api's client is uncached).
const serviceRefsLabel = "app.bex.co/has-service-refs"

// serviceRefRecord is one recorded reference: the consumer's env key, the
// target App's CR name in the same namespace, and the property (port or
// hostport).
type serviceRefRecord struct {
	Key      string `json:"key"`
	Target   string `json:"target"`
	Property string `json:"property"`
}

// env is the variable the record resolves to against target, and whether the
// target's port is known yet.
func (r serviceRefRecord) env(target *appv1alpha1.App) (appv1alpha1.EnvVar, bool) {
	// Recorded references carry a port; platform copies never are recorded.
	return hostRef{key: r.Key, property: r.Property}.env(target, "")
}

// serviceRefsOf reads a consumer's recorded references. A malformed annotation
// is an error, never an empty list: silently dropping it would stop updates.
func serviceRefsOf(app *appv1alpha1.App) ([]serviceRefRecord, error) {
	raw := app.Annotations[serviceRefsAnnotation]
	if raw == "" {
		return nil, nil
	}
	var refs []serviceRefRecord
	if err := json.Unmarshal([]byte(raw), &refs); err != nil {
		return nil, fmt.Errorf("service %s: malformed %s annotation: %w", app.Name, serviceRefsAnnotation, err)
	}
	return refs, nil
}

// encodeServiceRefs is the canonical annotation value: sorted by key, so an
// unchanged Blueprint re-records an identical string and writes nothing.
func encodeServiceRefs(refs []serviceRefRecord) string {
	if len(refs) == 0 {
		return ""
	}
	slices.SortFunc(refs, func(a, b serviceRefRecord) int { return strings.Compare(a.Key, b.Key) })
	b, _ := json.Marshal(refs) // a slice of plain strings cannot fail to marshal
	return string(b)
}

// stampServiceRefs records every applied service's port-carrying references on
// its App after both apply passes, when every target exists. It writes only
// metadata, and only when the recorded set changed, so it never opens a deploy
// and an unchanged re-sync stays a no-op.
func (s *Service) stampServiceRefs(ctx context.Context, st parsedStack, serviceApps map[string]*appv1alpha1.App, lookupTarget func(string) *appv1alpha1.App) error {
	for _, svc := range st.services {
		applied := serviceApps[svc.req.Name]
		if applied == nil {
			continue
		}
		var refs []serviceRefRecord
		for _, ref := range svc.hostRefs {
			if !ref.carriesPort() {
				continue
			}
			target := lookupTarget(ref.target)
			if target == nil { // unreachable: the apply refused unknown targets before any write
				return fmt.Errorf("blueprint apply: service %q: fromService target %q does not exist", svc.req.Name, ref.target)
			}
			refs = append(refs, serviceRefRecord{Key: ref.key, Target: target.Name, Property: ref.property})
		}
		// The apply passes never write the annotation, so the applied object's
		// copy is current: a steady-state re-sync reads nothing more.
		want := encodeServiceRefs(refs)
		if applied.Annotations[serviceRefsAnnotation] == want {
			continue
		}
		if err := requireDeployAuthority(ctx, s); err != nil {
			return err
		}
		// Locked: the operator writes status to the App the apply just
		// changed, so a conflict re-reads and records again rather than
		// failing the apply.
		if err := rollout.PatchAppLocked(ctx, s.Client, applied.DeepCopy(), func(a *appv1alpha1.App) error {
			if want == "" {
				delete(a.Annotations, serviceRefsAnnotation)
				delete(a.Labels, serviceRefsLabel)
				return nil
			}
			metav1.SetMetaDataAnnotation(&a.ObjectMeta, serviceRefsAnnotation, want)
			metav1.SetMetaDataLabel(&a.ObjectMeta, serviceRefsLabel, "true")
			return nil
		}); err != nil {
			return fmt.Errorf("blueprint apply: recording %q's service references: %w", svc.req.Name, err)
		}
	}
	return nil
}

// ServiceRefPropagator keeps every recorded fromService port reference equal
// to its target's serving port (w6/m148, ADR089 D4): the first discovery of an
// image-port service's ports, a later SetPort, and a promotion that moves the
// primary all reach the consumer without a Blueprint sync. Each change goes
// through the tracked, optimistically locked patch, so it opens one ordinary
// config_change deploy — or is held for a suspended consumer's next deploy —
// and two bex-api replicas cannot both apply it.
type ServiceRefPropagator struct {
	Svc *Service
	// Interval between ticks; non-positive means 20s.
	Interval time.Duration

	// missing is the consumer→target pairs whose target was gone last tick,
	// so a vanished target is logged once per episode, not every tick.
	missing map[string]bool
}

// Run drives the loop until ctx is canceled.
func (p *ServiceRefPropagator) Run(ctx context.Context) {
	if p == nil || p.Svc == nil || p.Svc.Client == nil {
		return
	}
	interval := p.Interval
	if interval <= 0 {
		interval = 20 * time.Second
	}
	core.Poll(ctx, "service references", interval, p.tick)
}

func (p *ServiceRefPropagator) tick(ctx context.Context) error {
	var consumers appv1alpha1.AppList
	if err := p.Svc.Client.List(ctx, &consumers, client.HasLabels{serviceRefsLabel}); err != nil {
		return err
	}
	// Resolve each target once per tick: several consumers often reference
	// one private service, and a listed consumer may itself be a target.
	cache := map[client.ObjectKey]*appv1alpha1.App{}
	for i := range consumers.Items {
		cache[client.ObjectKeyFromObject(&consumers.Items[i])] = &consumers.Items[i]
	}
	missing := map[string]bool{}
	for i := range consumers.Items {
		if ctx.Err() != nil {
			return nil
		}
		consumer := &consumers.Items[i]
		if consumer.DeletionTimestamp != nil {
			continue
		}
		lookup := func(name string) *appv1alpha1.App {
			key := client.ObjectKey{Namespace: consumer.Namespace, Name: name}
			target, seen := cache[key]
			if !seen {
				target = &appv1alpha1.App{}
				if err := p.Svc.Client.Get(ctx, key, target); err != nil {
					target = nil
				}
				cache[key] = target
			}
			if target == nil {
				pair := consumer.Namespace + "/" + consumer.Name + "->" + name
				if missing[pair] = true; !p.missing[pair] {
					log.Printf("service references: %s references %s, which is unavailable; keeping its last value", consumer.Name, name)
				}
			}
			return target
		}
		if _, err := p.Svc.propagateServiceRefs(ctx, consumer, lookup); err != nil {
			log.Printf("service references: %s/%s: %v", consumer.Namespace, consumer.Name, err)
		}
	}
	p.missing = missing
	return nil
}

// errServiceRefsCurrent ends a locked patch whose re-read found nothing left
// to write, so the losing replica of a race sends no empty patch.
var errServiceRefsCurrent = errors.New("service references already current")

// propagateServiceRefs patches consumer's recorded references to their
// targets' current values and reports whether it wrote. A target that is
// missing, or whose port is still unknown, leaves its variable untouched.
func (s *Service) propagateServiceRefs(ctx context.Context, consumer *appv1alpha1.App, lookup func(string) *appv1alpha1.App) (bool, error) {
	desired := func(a *appv1alpha1.App) (map[string]string, error) {
		refs, err := serviceRefsOf(a)
		if err != nil {
			return nil, err
		}
		want := map[string]string{}
		for _, ref := range refs {
			if target := lookup(ref.Target); target != nil {
				if env, ok := ref.env(target); ok {
					want[env.Name] = env.Value
				}
			}
		}
		return want, nil
	}
	want, err := desired(consumer)
	if err != nil || !envDiffers(consumer.Spec.Env, want) {
		return false, err
	}
	err = s.rollouts().PatchLocked(ctx, s.Client, consumer, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
		// A conflict re-reads a: decide again from what the winner left.
		want, err := desired(a)
		if err != nil {
			return err
		}
		if !envDiffers(a.Spec.Env, want) {
			return errServiceRefsCurrent
		}
		declared := make([]appv1alpha1.EnvVar, 0, len(want))
		for _, key := range slices.Sorted(maps.Keys(want)) {
			declared = append(declared, appv1alpha1.EnvVar{Name: key, Value: want[key]})
		}
		a.Spec.Env = mergeBlueprintEnv(a.Spec.Env, declared)
		resourcemeta.Touch(a, s.Now())
		return nil
	})
	if errors.Is(err, errServiceRefsCurrent) {
		return false, nil
	}
	return err == nil, err
}

// envDiffers reports whether any wanted literal is absent or different.
func envDiffers(env []appv1alpha1.EnvVar, want map[string]string) bool {
	for key, value := range want {
		i := slices.IndexFunc(env, func(e appv1alpha1.EnvVar) bool { return e.Name == key })
		if i < 0 || env[i].ValueFrom != nil || env[i].Value != value {
			return true
		}
	}
	return false
}
