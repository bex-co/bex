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

package objectgw

import (
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// The two calls the gateway makes to bex-api (ADR112 D4, D9). Both go to
// bex-api's cluster-internal listener, never the public one, and both are
// authenticated by one static bearer, BEX_OBJECT_STORAGE_GATEWAY_TOKEN: the
// shape of the ops-role and identity-claims verbs. It is a credential of its
// own, not the SSH gateway's BEX_SANDBOX_EXEC_SECRET, which also signs agent
// credential mints: an internet-facing server must not hold a key that can
// forge those.
const (
	// LookupPath is GET ?host=<request host>: which public bucket is this?
	LookupPath = "/internal/object-gateway/public-bucket"
	// EgressPath is POST: the bytes the gateway sent, per workspace, since its
	// last report.
	EgressPath = "/internal/object-gateway/egress"
	// PathPrefix is the subtree both verbs live under, so one mount covers them.
	PathPrefix = "/internal/object-gateway/"

	// BexAPIURL is where the gateway reaches bex-api's internal listener: a
	// constant, since the gateway's environment is credentials only (ADR112 D9).
	// Tests give a Config their own.
	BexAPIURL = "http://bex-api.bex-system.svc:8091"
)

// LookupAnswer is what bex-api says about a public bucket: where its objects
// live and the CORS rules to apply to them. A private, deleted or unknown bucket
// is a 404 with no body worth reading, so a visitor learns nothing from the
// difference.
type LookupAnswer struct {
	BucketID    string                   `json:"bucketId"`
	WorkspaceID string                   `json:"workspaceId"`
	Region      string                   `json:"region"`
	CORS        []objectstorage.CORSRule `json:"cors"`
}

// EgressReport is the body of an EgressPath call: bytes sent, by workspace id.
type EgressReport struct {
	Bytes map[string]int64 `json:"bytes"`
}

// Bounds of one egress report, enforced on both sides.
const (
	MaxReportWorkspaces = 10_000
	// MaxReportBodyBytes is the request body bex-api reads: 10,000 entries of
	// about 60 bytes, with room.
	MaxReportBodyBytes = 1 << 20
	// maxReportBytesPerWorkspace refuses a value no honest gateway sums in one
	// reporting interval (1 PiB), which would only be a bug or a forgery.
	maxReportBytesPerWorkspace = 1 << 50
)
