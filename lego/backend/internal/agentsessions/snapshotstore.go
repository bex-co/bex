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

package agentsessions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// ErrPartialS3SnapshotConfig is returned when some but not all of the required
// BEX_AGENT_SNAPSHOT_S3_* coordinates are set. A typo'd Secret key must fail
// startup rather than silently disable the Hibernated tier (w2/m77).
var ErrPartialS3SnapshotConfig = errors.New("BEX_AGENT_SNAPSHOT_S3_* is partially set: endpoint, bucket, access key, and secret key must all be set to enable hibernation, or all unset to keep it off")

// ErrForbiddenSnapshotBucket refuses the platform backup/state bucket. Agent
// snapshots need a dedicated SSE-enabled bucket (ADR059 D3 / ADR029).
var ErrForbiddenSnapshotBucket = errors.New("BEX_AGENT_SNAPSHOT_S3_BUCKET must not be bex-tfstate; use a dedicated snapshot bucket")

// SnapshotStore is the ADR059 D3 hibernation object store: it hands out
// short-lived presigned URLs so the sandbox streams its own encrypted snapshot
// to object storage on hibernate (a plain PUT — the bucket's default SSE
// encrypts at rest, ADR050) and a fresh sandbox fetches it back on rehydrate,
// and it deletes a snapshot on retention/cancel. bex-api holds the durable S3
// credentials and mints the URLs; **the tenant sandbox never receives a durable
// credential** — only a time-boxed, single-object URL, the Cloudflare-R2 pattern
// the ADR cites. nil ⇒ hibernation disabled (the Completer falls back to
// Terminate, byte-identical to w2/m67).
type SnapshotStore interface {
	// PrepareUpload returns the object ref (per-workspace prefix key) and a
	// short-lived presigned PUT URL the sandbox streams the snapshot to.
	PrepareUpload(ctx context.Context, workspaceID, sessionID string) (ref, putURL string, err error)
	// PrepareDownload returns a short-lived presigned GET URL for the given ref.
	PrepareDownload(ctx context.Context, ref string) (getURL string, err error)
	// Delete removes the snapshot object (retention/cancel); a missing object is
	// not an error (idempotent).
	Delete(ctx context.Context, ref string) error
}

// S3SnapshotStore is the production SnapshotStore over an S3-compatible endpoint.
// Snapshots live under `agent-snapshots/<workspace>/<session>-<mint>.tgz` — a
// per-workspace prefix, never an OCI registry (the D7 spike's rejected path).
// The bucket MUST have default server-side encryption enabled (ADR050 at-rest);
// the presigned PUT is a plain upload so the sandbox's `curl -T` needs no SSE
// header. The URL TTL bounds both the upload and the download windows.
type S3SnapshotStore struct {
	presign *s3.PresignClient
	client  *s3.Client
	bucket  string
	prefix  string
	ttl     time.Duration
	// nowFn is injectable so the ref mint is deterministic in tests (the SDK
	// forbids time.Now in workflow contexts; here it is only for the object key).
	nowFn func() time.Time
}

// S3SnapshotConfig is the resolved, non-secret-plus-secret coordinate set the
// composition root builds from BEX_AGENT_SNAPSHOT_S3_* before constructing the
// store. All-empty Bucket/Endpoint/credentials ⇒ NewS3SnapshotStore returns
// (nil, nil) and hibernation stays off. A partial set returns
// ErrPartialS3SnapshotConfig so startup fails closed.
type S3SnapshotConfig struct {
	Endpoint  string
	Bucket    string
	Region    string
	Prefix    string
	AccessKey string
	SecretKey string
	TTL       time.Duration
}

// Validate reports whether the Hibernated tier should arm. All-empty is a
// deliberate off; all four required coordinates set is on; anything in
// between is a configuration error. Region and prefix are optional (defaults
// apply at construct time). The platform state bucket is never a legal target.
func (c S3SnapshotConfig) Validate() error {
	type field struct {
		name, value string
	}
	required := []field{
		{"BEX_AGENT_SNAPSHOT_S3_ENDPOINT", c.Endpoint},
		{"BEX_AGENT_SNAPSHOT_S3_BUCKET", c.Bucket},
		{"BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY", c.AccessKey},
		{"BEX_AGENT_SNAPSHOT_S3_SECRET_KEY", c.SecretKey},
	}
	var set, missing []string
	for _, f := range required {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, f.name)
		} else {
			set = append(set, f.name)
		}
	}
	switch {
	case len(set) == 0:
		return nil
	case len(missing) > 0:
		return fmt.Errorf("%w (set: %s; missing: %s)", ErrPartialS3SnapshotConfig, strings.Join(set, ", "), strings.Join(missing, ", "))
	}
	if strings.EqualFold(strings.TrimSpace(c.Bucket), "bex-tfstate") {
		return ErrForbiddenSnapshotBucket
	}
	return nil
}

// NewS3SnapshotStore builds the store. A fully empty config returns (nil, nil)
// (hibernation disabled — the safe default). A partial config or the platform
// state bucket returns an error the composition root treats as fatal. TTL
// defaults to 15m and the prefix to "agent-snapshots".
func NewS3SnapshotStore(cfg S3SnapshotConfig) (*S3SnapshotStore, error) {
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.AccessKey = strings.TrimSpace(cfg.AccessKey)
	cfg.SecretKey = strings.TrimSpace(cfg.SecretKey)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Prefix = strings.TrimSpace(cfg.Prefix)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Bucket == "" {
		return nil, nil
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "agent-snapshots"
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	provider := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")
	client := s3.NewFromConfig(aws.Config{Region: region, Credentials: aws.NewCredentialsCache(provider)}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
	return &S3SnapshotStore{
		presign: s3.NewPresignClient(client),
		client:  client,
		bucket:  cfg.Bucket,
		prefix:  prefix,
		ttl:     ttl,
		nowFn:   time.Now,
	}, nil
}

func (s *S3SnapshotStore) now() time.Time {
	if s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now()
}

// snapshotKey is the per-workspace-prefixed object key. The trailing epoch keeps
// a re-hibernation of the same session from overwriting a still-referenced blob
// before the old ref is deleted (the store never mutates an existing object).
func (s *S3SnapshotStore) snapshotKey(workspaceID, sessionID string) string {
	return fmt.Sprintf("%s/%s/%s-%d.tgz", s.prefix, workspaceID, sessionID, s.now().UnixNano())
}

func (s *S3SnapshotStore) PrepareUpload(ctx context.Context, workspaceID, sessionID string) (string, string, error) {
	if s == nil {
		return "", "", core.ErrAgentSessionsUnavailable
	}
	key := s.snapshotKey(workspaceID, sessionID)
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(o *s3.PresignOptions) { o.Expires = s.ttl })
	if err != nil {
		return "", "", fmt.Errorf("presign snapshot upload: %w", err)
	}
	return key, req.URL, nil
}

func (s *S3SnapshotStore) PrepareDownload(ctx context.Context, ref string) (string, error) {
	if s == nil {
		return "", core.ErrAgentSessionsUnavailable
	}
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(ref),
	}, func(o *s3.PresignOptions) { o.Expires = s.ttl })
	if err != nil {
		return "", fmt.Errorf("presign snapshot download: %w", err)
	}
	return req.URL, nil
}

func (s *S3SnapshotStore) Delete(ctx context.Context, ref string) error {
	if s == nil || ref == "" {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(ref),
	})
	return err
}
