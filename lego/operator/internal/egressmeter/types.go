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

package egressmeter

import (
	"bytes"
	"fmt"
)

const labelBytes = 64

// ResourceKey is the durable, low-cardinality attribution carried in the BPF
// maps. It intentionally excludes pod UID and workspace: one Prometheus series
// identifies one App on one node/source instance, while the Pod-IP maps supply
// the short-lived packet identity before source NAT.
type ResourceKey struct {
	AppID     [labelBytes]byte
	Namespace [labelBytes]byte
}

func NewResourceKey(namespace, appID string) (ResourceKey, error) {
	if namespace == "" || appID == "" {
		return ResourceKey{}, fmt.Errorf("namespace and app id are required")
	}
	if len(namespace) >= labelBytes || len(appID) >= labelBytes {
		return ResourceKey{}, fmt.Errorf("namespace or app id exceeds %d bytes", labelBytes-1)
	}
	var key ResourceKey
	copy(key.Namespace[:], namespace)
	copy(key.AppID[:], appID)
	return key, nil
}

func (k ResourceKey) Labels() (namespace, appID string) {
	trim := func(v []byte) string {
		if i := bytes.IndexByte(v, 0); i >= 0 {
			v = v[:i]
		}
		return string(v)
	}
	return trim(k.Namespace[:]), trim(k.AppID[:])
}
