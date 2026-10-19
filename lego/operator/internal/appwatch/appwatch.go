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

// Package appwatch keeps a watch-backed, in-memory copy of every App and
// signals each change, so the public host indexes in the activator and the
// static-server learn a new or changed service within one watch event instead
// of one poll interval (w4/m235).
package appwatch

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	crcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Start syncs an App cache (every namespace, or only namespace when set) and
// returns it with a channel that receives after an App that relevant accepts
// (every App when nil) is added, changed or deleted. Signals coalesce: a reader
// that is busy sees one pending signal, not one per event, and rebuilds from
// the cache, which costs no API call.
func Start(ctx context.Context, cfg *rest.Config, scheme *runtime.Scheme, namespace string, relevant func(*appv1alpha1.App) bool) (client.Reader, <-chan struct{}, error) {
	opts := crcache.Options{Scheme: scheme, DefaultTransform: crcache.TransformStripManagedFields()}
	if namespace != "" {
		opts.DefaultNamespaces = map[string]crcache.Config{namespace: {}}
	}
	apps, err := crcache.New(cfg, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("appwatch: new cache: %w", err)
	}
	informer, err := apps.GetInformer(ctx, &appv1alpha1.App{})
	if err != nil {
		return nil, nil, fmt.Errorf("appwatch: app informer: %w", err)
	}
	changed := make(chan struct{}, 1)
	if _, err := informer.AddEventHandler(signaller(changed, relevant)); err != nil {
		return nil, nil, fmt.Errorf("appwatch: app handler: %w", err)
	}
	// A cache that cannot start or list (missing RBAC, no API server) fails
	// the process loudly instead of hanging before its listener opens.
	syncCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	started := make(chan error, 1)
	go func() {
		err := apps.Start(ctx)
		started <- err
		cancel()
	}()
	if !apps.WaitForCacheSync(syncCtx) {
		select {
		case err := <-started:
			if err != nil {
				return nil, nil, fmt.Errorf("appwatch: start app cache: %w", err)
			}
		default:
		}
		return nil, nil, fmt.Errorf("appwatch: app cache did not sync within %v", syncTimeout)
	}
	return apps, changed, nil
}

// signaller sends one coalesced signal on changed for each event that touches
// an App relevant accepts (every App when nil); an update counts when either
// its old or new state is relevant, so an App leaving the set is seen too.
func signaller(changed chan<- struct{}, relevant func(*appv1alpha1.App) bool) toolscache.ResourceEventHandlerFuncs {
	signal := func(objs ...any) {
		for _, obj := range objs {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			if app, ok := obj.(*appv1alpha1.App); !ok || relevant == nil || relevant(app) {
				select {
				case changed <- struct{}{}:
				default:
				}
				return
			}
		}
	}
	return toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { signal(obj) },
		UpdateFunc: func(old, obj any) { signal(old, obj) },
		DeleteFunc: func(obj any) { signal(obj) },
	}
}

// syncTimeout bounds the first full App list.
const syncTimeout = 2 * time.Minute

// rebuildInterval bounds how often a reader rebuilds while Apps change
// continuously (a build polls its status every few seconds), well inside the
// operator's two-second static handover.
const rebuildInterval = 250 * time.Millisecond

// Loop calls rebuild now, then after each changed signal and each backstop
// tick until ctx is cancelled, at most once per rebuildInterval.
func Loop(ctx context.Context, backstop time.Duration, changed <-chan struct{}, rebuild func()) {
	t := time.NewTicker(backstop)
	defer t.Stop()
	for {
		rebuild()
		select {
		case <-ctx.Done():
			return
		case <-time.After(rebuildInterval):
		}
		select {
		case <-ctx.Done():
			return
		case <-changed:
		case <-t.C:
		}
	}
}
