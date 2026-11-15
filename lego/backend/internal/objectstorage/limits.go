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

import "time"

// The caps below are bex's own (ADR112 D8). No Render or Cloudflare value was
// found for any of them, so each is a dated, unverified entry in ADR018.
// Changing one is a reviewed code change with a new dated entry; none is an
// environment variable (ADR112 D9).
const (
	// MaxWorkspaceBytes and MaxWorkspaceObjects bound one workspace in one
	// region. The object cap exists so one hourly sweep finishes inside its hour.
	MaxWorkspaceBytes   int64 = 100 * 1000 * 1000 * 1000
	MaxWorkspaceObjects int64 = 1_000_000

	// MaxUserBuckets is the number of buckets a workspace may create; its
	// render-<region> system buckets do not count.
	MaxUserBuckets = 1000

	// MaxObjectBytesPresigned is the largest object Render's routes mint a
	// URL for: the S3 single-PUT ceiling. Wasabi's own limit is unverified.
	MaxObjectBytesPresigned int64 = 5 << 30
	// MaxObjectBytesStreamed is the largest object the Cloudflare-shaped routes
	// take: wrangler's own cap, because its bytes stream through bex-api.
	MaxObjectBytesStreamed int64 = 300 << 20

	// MaxConcurrentStreamsPerWorkspace bounds the uploads and downloads one
	// workspace may have in flight through bex-api.
	MaxConcurrentStreamsPerWorkspace = 8
	// MaxConcurrentStreamsPerReplica bounds all workspaces together on one
	// bex-api replica, so a crowd of workspaces cannot exhaust its connections
	// and memory. ADR112 gives no value; this is bex's own.
	MaxConcurrentStreamsPerReplica = 64

	// MintBurst and MintPerSecond are a workspace's token bucket for minting
	// presigned URLs. On Contract A the bytes bypass bex-api, so the gate sees
	// only the size a caller says it will upload; this rate bounds how far
	// uploads can overshoot a cap between two sweeps (ADR112 D8). ADR112 gives
	// no value; this is bex's own.
	MintBurst     = 30
	MintPerSecond = 5

	// MaxCORSRules and MaxCORSBytes bound one bucket's CORS configuration;
	// MaxLifecycleRules bounds its lifecycle rules.
	MaxCORSRules      = 100
	MaxCORSBytes      = 64 << 10
	MaxLifecycleRules = 1000

	// MaxKeyBytes is the longest object key.
	MaxKeyBytes = 1024

	// ListDefaultLimit and ListMaxLimit are Render's list page: the default is
	// from the Python SDK's docstring; the cap is the S3 page size, because
	// the CLI's own help example asks for 200 and a lower cap would truncate
	// silently.
	ListDefaultLimit = 20
	ListMaxLimit     = 1000

	// PresignTTL is how long a presigned URL stays valid.
	PresignTTL = 15 * time.Minute
)
