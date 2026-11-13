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

package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The keys of the Secret the operator mints for the sandbox storage meter's Zot
// users (lego/operator/internal/registry/sandbox_meter_creds.go, w22/m321,
// w2/m358); the two modules cannot share a constant, so these must match it.
const (
	registrySecretReaderUserKey     = "reader-username"
	registrySecretReaderPasswordKey = "reader-password"
	registrySecretReaperUserKey     = "reaper-username"
	registrySecretReaperPasswordKey = "reaper-password"
	// The importer copies imported sandbox images into the snapshot
	// repositories (w2/m358).
	registrySecretImporterUserKey     = "importer-username"
	registrySecretImporterPasswordKey = "importer-password"

	// registrySecretTTL bounds how stale a cached credential may get: the Secret
	// is read at most this often, and a rotated password is picked up within it.
	registrySecretTTL = 5 * time.Minute
)

// parseSecretRef splits "<namespace>/<name>".
func parseSecretRef(ref string) (client.ObjectKey, error) {
	ns, name, ok := strings.Cut(ref, "/")
	if !ok || ns == "" || name == "" || strings.Contains(name, "/") {
		return client.ObjectKey{}, fmt.Errorf("%q is not <namespace>/<name>", ref)
	}
	return client.ObjectKey{Namespace: ns, Name: name}, nil
}

// secretCredentials reads a user and password from one key pair of a Secret,
// cached for registrySecretTTL. A failed read serves the last good value while
// there is one, so a brief API outage does not stop the meter.
func secretCredentials(c client.Reader, key client.ObjectKey, userKey, passwordKey string, now func() time.Time) func(context.Context) (string, string, error) {
	var (
		mu         sync.Mutex
		user, pass string
		fetched    time.Time
	)
	return func(ctx context.Context) (string, string, error) {
		mu.Lock()
		defer mu.Unlock()
		if user != "" && now().Sub(fetched) < registrySecretTTL {
			return user, pass, nil
		}
		var sec corev1.Secret
		if err := c.Get(ctx, key, &sec); err != nil {
			if user != "" {
				return user, pass, nil
			}
			return "", "", fmt.Errorf("read secret %s: %w", key, err)
		}
		u, p := string(sec.Data[userKey]), string(sec.Data[passwordKey])
		if u == "" || p == "" {
			return "", "", fmt.Errorf("secret %s has no %s/%s yet", key, userKey, passwordKey)
		}
		user, pass, fetched = u, p, now()
		return user, pass, nil
	}
}
