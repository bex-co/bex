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
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// CheckedDeployWorker recovers source-bound pending work and release receipts
// after API restart; suite delivery is only the faster path to the same verb.
type CheckedDeployWorker struct {
	Svc      *Service
	Interval time.Duration
	after    string
}

func (w *CheckedDeployWorker) Run(ctx context.Context) {
	if w == nil || w.Svc == nil || w.Svc.Client == nil {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	core.Poll(ctx, "checked deploys", interval, w.tick)
}

func (w *CheckedDeployWorker) tick(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var apps appv1alpha1.AppList
	if err := w.Svc.Client.List(ctx, &apps); err != nil {
		return err
	}
	pending := make([]*appv1alpha1.App, 0)
	for i := range apps.Items {
		a := &apps.Items[i]
		if a.Annotations[annotChecksPendingCommit] != "" || a.Annotations[annotChecksDispatch] != "" {
			pending = append(pending, a)
		}
	}
	key := func(a *appv1alpha1.App) string { return a.Namespace + "/" + a.Name }
	slices.SortFunc(pending, func(a, b *appv1alpha1.App) int { return strings.Compare(key(a), key(b)) })
	start, _ := slices.BinarySearchFunc(pending, w.after, func(a *appv1alpha1.App, after string) int { return strings.Compare(key(a), after) })
	if start < len(pending) && key(pending[start]) == w.after {
		start++
	}
	// Rotate a bounded batch: failing/slow services must not monopolize every
	// poll and strand later accepted work indefinitely.
	for i := 0; i < min(len(pending), 50); i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		a := pending[(start+i)%len(pending)]
		w.after = key(a)
		sha := a.Annotations[annotChecksPendingCommit]
		if _, err := w.Svc.dispatchCheckedCommit(ctx, a, store.CommitInfo{Hash: sha}); err != nil {
			log.Printf("checked deploy recovery %s: %v", a.Name, err)
		}
	}
	return nil
}
