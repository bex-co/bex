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

package objectstorage

import "github.com/bex-co/bex/lego/backend/internal/core"

// The audit verbs of the object store. Render's /v1/objects routes and
// wrangler's Cloudflare-shaped routes authorize in their HTTP adapters
// (internal/api), where the stack frame core.Base derives a verb from is a
// route closure, so each route names its verb here and passes it to
// core.Base.AuthorizeWorkspaceTargetAs. Both contracts share a verb for the same
// action: a put through either is objectstorage.PutObject.
//
// Render's audit-log enum has no object or bucket event, so none maps onto one:
// the audit log reports these as bex's own "<feature>.<Action>" value
// (docs/render-artifacts/audit-logs-api.md).
//
// Every row names its bucket, and an object row its key, in the audit target
// (core.BucketTarget, core.ObjectTarget; ADR112 D6, w21/094). A write relation
// records the allowed call and the denied one, a read relation only the denied
// one (core.writeRelations, core.readRelations), so the list and info verbs
// appear in the log when a caller was refused. GetObject is the one read that
// also records a success, once per request (core.RecordObjectViewed).
const (
	// Objects.
	AuditVerbListObjects  = "objectstorage.ListObjects"
	AuditVerbPutObject    = "objectstorage.PutObject"
	AuditVerbGetObject    = core.AuditVerbGetObject
	AuditVerbDeleteObject = "objectstorage.DeleteObject"

	// Buckets.
	AuditVerbListBuckets    = "objectstorage.ListBuckets"
	AuditVerbCreateBucket   = "objectstorage.CreateBucket"
	AuditVerbGetBucket      = "objectstorage.GetBucket"
	AuditVerbUpdateBucket   = "objectstorage.UpdateBucket"
	AuditVerbDeleteBucket   = "objectstorage.DeleteBucket"
	AuditVerbGetBucketUsage = "objectstorage.GetBucketUsage"

	// A bucket's configuration.
	AuditVerbGetBucketCORS      = "objectstorage.GetBucketCORS"
	AuditVerbSetBucketCORS      = "objectstorage.SetBucketCORS"
	AuditVerbDeleteBucketCORS   = "objectstorage.DeleteBucketCORS"
	AuditVerbGetBucketLifecycle = "objectstorage.GetBucketLifecycle"
	AuditVerbSetBucketLifecycle = "objectstorage.SetBucketLifecycle"
	AuditVerbGetBucketPublic    = "objectstorage.GetBucketPublic"
	AuditVerbSetBucketPublic    = "objectstorage.SetBucketPublic"
	AuditVerbListCustomDomains  = "objectstorage.ListCustomDomains"
	AuditVerbNotOffered         = "objectstorage.NotOffered"

	// S3 access keys (ADR112 D3). Creating and revoking one are write verbs; a
	// listing is a read, so only a refusal of it is recorded.
	AuditVerbCreateAccessKey = "objectstorage.CreateAccessKey"
	AuditVerbListAccessKeys  = "objectstorage.ListAccessKeys"
	AuditVerbRevokeAccessKey = "objectstorage.RevokeAccessKey"
)
