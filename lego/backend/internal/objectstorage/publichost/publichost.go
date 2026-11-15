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

// Package publichost is the one place that knows what a public bucket's host
// looks like (ADR112 D4): "pub-<xid>" is a single DNS label under
// BEX_BASE_DOMAIN, where <xid> is the xid of the bucket's "obk-<xid>" id.
//
// It is a leaf (it imports only internal/id) so that bex-api's routes, the
// gateway, and the store that mints service slugs can all agree on the shape
// without importing each other.
package publichost

import (
	"strings"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Prefix leads the label of every public bucket host.
const Prefix = "pub-"

const bucketPrefix = "obk-"

// Label is the host label of a bucket's public host, "pub-<xid>". ok is false
// when bucketID is not a well-formed object bucket id.
func Label(bucketID string) (label string, ok bool) {
	if kind, valid := ids.KindOf(bucketID); !valid || kind != ids.ObjectBucket {
		return "", false
	}
	return Prefix + strings.TrimPrefix(bucketID, bucketPrefix), true
}

// BucketID is the bucket id a host label names, the inverse of Label. ok is
// false for anything that is not exactly "pub-" and a well-formed xid, so a
// label is never a way to name some other id.
func BucketID(label string) (bucketID string, ok bool) {
	rest, found := strings.CutPrefix(label, Prefix)
	if !found {
		return "", false
	}
	id := bucketPrefix + rest
	if kind, valid := ids.KindOf(id); !valid || kind != ids.ObjectBucket {
		return "", false
	}
	return id, true
}

// Host is the public host of a bucket under baseDomain. ok is false when the
// bucket id is malformed or baseDomain is empty: without a hosting domain there
// is no host to serve.
func Host(bucketID, baseDomain string) (host string, ok bool) {
	label, valid := Label(bucketID)
	if !valid || baseDomain == "" {
		return "", false
	}
	return label + "." + baseDomain, true
}

// NormalizeHost lower-cases a request host and drops its port and trailing dot,
// so every spelling of one host is one key.
func NormalizeHost(host string) string {
	host = strings.ToLower(host)
	if i := strings.LastIndexByte(host, ':'); i >= 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	return strings.TrimSuffix(host, ".")
}

// BucketIDOfHost is the bucket id a request host names under baseDomain. It
// accepts one DNS label under the domain and nothing else: a port, a trailing
// dot and letter case are normalized, but a deeper name, the apex or another
// domain is not a public bucket host.
func BucketIDOfHost(host, baseDomain string) (bucketID string, ok bool) {
	if baseDomain == "" {
		return "", false
	}
	label, found := strings.CutSuffix(NormalizeHost(host), "."+strings.ToLower(baseDomain))
	if !found {
		return "", false
	}
	return BucketID(label) // a label with a dot in it is not the shape of one
}

// APILabel is the host label of the direct (not Cloudflare-proxied) host of the
// Cloudflare-shaped object storage API, "objects" (objects.<base domain>). The
// proxy in front of api.<base domain> refuses a body over 100 MiB, so the
// wrangler routes that take up to 300 MiB are also routed here
// (lego/operator/config/api/ingressroute-objects.yaml, ADR112 D3).
const APILabel = "objects"

// Reserved reports whether a service slug is a label the platform serves objects
// at: the shape of a public bucket host (pub-<xid>) or APILabel. A slug is the
// label of the service's own host, so none of these may be one a service can
// take. Only these exact labels are reserved: a service called "pub-site"
// keeps its name, as on Render.
func Reserved(slug string) bool {
	_, ok := BucketID(slug)
	return ok || slug == APILabel
}
