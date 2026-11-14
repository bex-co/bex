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

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// zotStorageActiveAnnotation records, on the Zot config Secret, the storage
// block a Zot restart has loaded. Zot reads its config only when it starts
// (verify.go), and a storage change leaves every credential valid, so nothing
// else restarts it: the retention rules in the Secret and the rules Zot runs
// could differ until some unrelated restart. The corrected `live-` rule order
// would have reached the Secret and not the registry (w8/m156).
const zotStorageActiveAnnotation = "bex.co/zot-storage-active"

// storageEpoch names the canonical storage block by a short hash of its JSON
// (encoding/json writes map keys in order, so equal blocks hash alike).
func (c *Creds) storageEpoch() string {
	raw, _ := json.Marshal(c.canonicalStorage())
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// ensureStorageActive restarts Zot once after the storage block of its config
// changes, and records that on the config Secret. The caller has just brought
// the Secret's storage block to the canonical one.
//
// The record is written after the restart, never before: a restart that fails
// is then tried again at the next reconcile, where one recorded first would
// leave the old rules running. It is best effort, like a reload nudge: a
// failure here must not fail the App's reconcile.
func (c *Creds) ensureStorageActive(ctx context.Context) {
	c.mu.Lock()
	active := c.storageActive
	c.mu.Unlock()
	if active {
		return
	}
	log := logf.FromContext(ctx)
	epoch := c.storageEpoch()
	var config corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: c.ZotNamespace, Name: c.ConfigName}, &config); err != nil {
		log.Info("zot storage activation deferred", "error", err.Error())
		return
	}
	if config.Annotations[zotStorageActiveAnnotation] != epoch {
		if !c.claimBounce(time.Now()) {
			return
		}
		if err := c.bounceZot(ctx); err != nil {
			log.Error(err, "zot restart to load its storage settings failed; retrying next reconcile")
			return
		}
		base := config.DeepCopy()
		if config.Annotations == nil {
			config.Annotations = map[string]string{}
		}
		config.Annotations[zotStorageActiveAnnotation] = epoch
		if err := c.Client.Patch(ctx, &config, client.MergeFrom(base)); err != nil {
			log.Error(err, "recording the zot storage restart failed; it will restart again after the cooldown")
			return
		}
		log.Info("restarted zot to load its storage settings", "epoch", epoch)
	}
	c.mu.Lock()
	c.storageActive = true
	c.mu.Unlock()
}
