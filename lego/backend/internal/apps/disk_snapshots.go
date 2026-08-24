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
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
	"github.com/bex-co/bex/lego/backend/internal/snapshotticket"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Disk snapshots (docs/ADR082-persistent-disks.md D5), the read and restore
// half. The operator writes them nightly; this is where a tenant sees what
// exists and asks for one back.
//
// Render's own warning is carried verbatim into every description here, because
// it is the thing a user most needs to know before clicking: a disk restore is
// not a database backup, and restoring one under a running database can produce
// a corrupted state.

// diskSnapshotSuffix must match the operator's disksnapshot.SnapshotSuffix. The
// two modules cannot import each other (operator → types ← backend), so this is
// a hand-synced constant like labelWorkspace, and the listing filters on it so
// a listing can never present some other object as a restorable snapshot.
const diskSnapshotSuffix = ".tar.gz.age"

// DiskSnapshotView is one stored snapshot, in Render's shape.
type DiskSnapshotView struct {
	// CreatedAt is when the snapshot was taken (parsed from the object name,
	// which the operator writes in UTC RFC3339 so it sorts in time order).
	CreatedAt time.Time `json:"createdAt"`
	// SnapshotKey is the signed handle a restore takes back. Render's keys
	// "expire after 24 hours" and so do these; re-list to get a fresh one.
	SnapshotKey string `json:"snapshotKey"`
	// InstanceID is Render's per-instance field. A bex disk is attached to at
	// most one instance, so it names the owning service — the shape is
	// preserved even though the multi-instance case it exists for cannot arise.
	InstanceID string `json:"instanceId"`
}

// DiskSnapshotLister reads the snapshot objects for one disk. It is an
// interface so the verbs are testable without an object store, and so a
// deployment with no store configured degrades to a clear 503 rather than a
// panic.
type DiskSnapshotLister interface {
	ListSnapshots(ctx context.Context, prefix string) ([]DiskSnapshotObject, error)
}

// DiskSnapshotsNotConfiguredCode marks the one 503 this feature can return that
// is not a failure: the deployment never wired an object store for disk
// snapshots (BEX_DISK_SNAPSHOT_*, ADR082 D5), so there is nothing to list and
// nothing to restore from.
//
// It is a CODE rather than a message because that is the only thing a client
// can branch on safely. A dashboard matching on prose has to guess, and a
// message that gets sanitized in transit degrades to "internal error" — which
// tells a tenant their service is broken when in fact an operator simply has
// not turned the feature on.
const DiskSnapshotsNotConfiguredCode = "DISK_SNAPSHOTS_NOT_CONFIGURED"

func errDiskSnapshotsNotConfigured() error {
	return core.NewUnavailableError(
		DiskSnapshotsNotConfiguredCode,
		"disk snapshots are not configured for this deployment",
		nil,
	)
}

// DiskSnapshotObject is one object as the store reports it.
type DiskSnapshotObject struct {
	Key       string
	CreatedAt time.Time
}

// ListDiskSnapshots returns a disk's snapshots, newest first, each with a
// freshly signed 24-hour key.
func (s *Service) ListDiskSnapshots(ctx context.Context, diskID string) ([]DiskSnapshotView, error) {
	disk, a, err := s.authorizeDisk(ctx, core.RelCanView, diskID)
	if err != nil {
		return nil, err
	}
	if s.DiskSnapshots == nil || len(s.SnapshotSecret) == 0 {
		return nil, errDiskSnapshotsNotConfigured()
	}
	objects, err := s.DiskSnapshots.ListSnapshots(ctx, diskSnapshotPrefix(a))
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	views := make([]DiskSnapshotView, 0, len(objects))
	for _, obj := range objects {
		if !strings.HasSuffix(obj.Key, diskSnapshotSuffix) {
			continue
		}
		key, err := snapshotticket.Mint(s.SnapshotSecret, disk.ID, obj.Key, s.Now())
		if err != nil {
			return nil, err
		}
		views = append(views, DiskSnapshotView{
			CreatedAt:   snapshotCreatedAt(obj),
			SnapshotKey: key,
			InstanceID:  disk.AppID,
		})
	}
	// Newest first: the snapshot a caller almost always wants is the last one.
	sort.Slice(views, func(i, j int) bool { return views[i].CreatedAt.After(views[j].CreatedAt) })
	return views, nil
}

// snapshotCreatedAt prefers the timestamp encoded in the object name over the
// store's LastModified: the name is when the snapshot was TAKEN, while
// LastModified moves if an object is ever copied or re-uploaded.
func snapshotCreatedAt(obj DiskSnapshotObject) time.Time {
	name := obj.Key
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if at, err := time.Parse(time.RFC3339, strings.TrimSuffix(name, diskSnapshotSuffix)); err == nil {
		return at.UTC()
	}
	return obj.CreatedAt.UTC()
}

// diskSnapshotPrefix must agree with the operator's disksnapshot.DiskPrefix.
// It is keyed on the App rather than the disk id because the operator's Job
// only knows the App it runs beside — a disk id lives in the control-plane row,
// which the DB-free operator never reads.
func diskSnapshotPrefix(a *appv1alpha1.App) string {
	workspace := a.Labels[core.LabelTenant]
	if workspace == "" {
		workspace = a.Namespace
	}
	return workspace + "/" + a.Name + "/"
}

// RestoreDiskSnapshot replaces a disk's contents with one of its snapshots and
// returns the disk it accepted the restore for.
//
// Render's words, which the surfaces repeat: "This operation is irreversible" —
// everything written after the snapshot is lost, and the service restarts.
//
// The work is asynchronous: the operator scales the service down, runs the
// restore Job against the freed volume, and brings it back, which takes far
// longer than a request. bex nonetheless answers Render's documented **200 with
// the disk** rather than a bare 202 (w1/m88/t002). Returning the resource does
// not claim the restore finished — it names what is being restored, which is
// the only thing a caller can act on — and a Render client's generated code
// decodes a disk here, so a bodyless 202 leaves it holding nothing.
func (s *Service) RestoreDiskSnapshot(ctx context.Context, diskID, snapshotKey string) (DiskView, error) {
	disk, a, err := s.authorizeDisk(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, diskID)
	if err != nil {
		return DiskView{}, err
	}
	if len(s.SnapshotSecret) == 0 {
		return DiskView{}, errDiskSnapshotsNotConfigured()
	}
	if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		return DiskView{}, err
	}
	claims, err := snapshotticket.Verify(s.SnapshotSecret, strings.TrimSpace(snapshotKey), s.Now())
	if err != nil {
		// Expired, forged, or tampered — all the same answer, and crucially the
		// service has not been touched: nothing scales down before the key is
		// known good.
		return DiskView{}, fmt.Errorf("%w: snapshotKey is not valid (keys expire after 24 hours; list the disk's snapshots again)", core.ErrBadRequest)
	}
	if claims.DiskID != disk.ID {
		// A well-signed key for a DIFFERENT disk. Refused here rather than
		// range-checked in the Job, so a mis-issued key can never cross disks.
		return DiskView{}, fmt.Errorf("%w: that snapshot belongs to another disk", core.ErrBadRequest)
	}
	if _, err := s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		// The intent is a verb-as-value on the spec, like restartedAt: the
		// operator sees a request it has not served yet and runs the restore.
		a.Spec.Disk.RestoreSnapshot = claims.Object
	}); err != nil {
		return DiskView{}, fmt.Errorf("request restore: %w", err)
	}
	return diskView(disk), nil
}

// --- REST ---

func (s *Service) registerDiskSnapshotRoutes(mux *http.ServeMux) {
	// Both status codes are Render's, taken from the pinned contract rather
	// than from what reads best (w1/m88/t002). list-snapshots is documented
	// **201** even though it is a GET that creates nothing — that is Render's
	// own spec, and a generated client switches on the code, so answering a
	// tidier 200 would leave its 201 branch nil and its result empty.
	mux.HandleFunc("GET /v1/disks/{diskId}/snapshots", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		return s.ListDiskSnapshots(r.Context(), r.PathValue("diskId"))
	}))
	// restore-snapshot is documented 200 with the disk object. The work stays
	// asynchronous; returning the resource names what is being restored rather
	// than claiming it finished.
	mux.HandleFunc("POST /v1/disks/{diskId}/snapshots/restore", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[restoreSnapshotRequest](r)
		if err != nil {
			return nil, err
		}
		return s.RestoreDiskSnapshot(r.Context(), r.PathValue("diskId"), req.SnapshotKey)
	}))
}

type restoreSnapshotRequest struct {
	SnapshotKey string `json:"snapshotKey"`
	// InstanceID is accepted for Render wire-compatibility and ignored: a bex
	// disk is attached to at most one instance, so there is nothing to select.
	InstanceID string `json:"instanceId,omitempty"`
}

// --- GraphQL ---

var diskSnapshotGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DiskSnapshot",
	Fields: graphql.Fields{
		"createdAt":   gqlutil.StrField(func(v DiskSnapshotView) any { return v.CreatedAt.UTC().Format(time.RFC3339) }),
		"snapshotKey": gqlutil.StrField(func(v DiskSnapshotView) any { return v.SnapshotKey }),
		"instanceId":  gqlutil.StrField(func(v DiskSnapshotView) any { return v.InstanceID }),
	},
})

func (s *Service) diskSnapshotGQLQueryFields() graphql.Fields {
	return graphql.Fields{
		"diskSnapshots": &graphql.Field{
			Type: graphql.NewList(diskSnapshotGQLType),
			Args: graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListDiskSnapshots(p.Context, p.Args["id"].(string))
			},
		},
	}
}

func (s *Service) diskSnapshotGQLMutationFields() graphql.Fields {
	return graphql.Fields{
		"restoreDiskSnapshot": &graphql.Field{
			// Returns the disk, matching REST and MCP. A bare Boolean told the
			// caller only that nothing threw (w1/m88/t002).
			Type: diskGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          gqlutil.ReqArg(graphql.String),
				"snapshotKey": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RestoreDiskSnapshot(p.Context, p.Args["id"].(string), p.Args["snapshotKey"].(string))
			},
		},
	}
}

// --- MCP ---

type restoreSnapshotArgs struct {
	DiskID      string `json:"diskId" jsonschema:"the disk id (dsk-...)"`
	SnapshotKey string `json:"snapshotKey" jsonschema:"a snapshotKey from list_disk_snapshots; keys expire after 24 hours"`
}

type listSnapshotsResult struct {
	Snapshots []DiskSnapshotView `json:"snapshots"`
}

type restoreSnapshotResult struct {
	Restoring bool `json:"restoring"`
	// Disk is the disk the restore was accepted for — the same object REST and
	// GraphQL hand back, so an agent reading this tool's result sees what the
	// other two surfaces show rather than a bare flag.
	Disk *DiskView `json:"disk,omitempty"`
}

func (s *Service) registerDiskSnapshotTools(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name: "list_disk_snapshots",
		Description: "List a persistent disk's daily snapshots, newest first. Each snapshotKey expires " +
			"after 24 hours; list again to get a fresh one.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in diskIDArgs) (*mcp.CallToolResult, listSnapshotsResult, error) {
		snapshots, err := s.ListDiskSnapshots(ctx, in.DiskID)
		if err != nil {
			return nil, listSnapshotsResult{}, err
		}
		return nil, listSnapshotsResult{Snapshots: snapshots}, nil
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name: "restore_disk_snapshot",
		Description: "Restore a persistent disk to one of its snapshots. IRREVERSIBLE: everything written " +
			"after the snapshot is lost and the service restarts. Do not use this to recover a database " +
			"running on the disk — a file-level restore of live database files can produce a corrupted " +
			"state; use that database's own dump/restore instead.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in restoreSnapshotArgs) (*mcp.CallToolResult, restoreSnapshotResult, error) {
		disk, err := s.RestoreDiskSnapshot(ctx, in.DiskID, in.SnapshotKey)
		if err != nil {
			return nil, restoreSnapshotResult{}, err
		}
		return nil, restoreSnapshotResult{Restoring: true, Disk: &disk}, nil
	})
}

// --- S3-backed lister ---

// S3DiskSnapshotLister lists snapshot objects straight from the store the
// operator writes them to. The operator has its own client for the same bucket;
// the two modules cannot import each other (operator → types ← backend), so
// this is the read half only — no Put, no Delete — which keeps the duplication
// to a listing.
type S3DiskSnapshotLister struct {
	client *s3.Client
	bucket string
	prefix string
}

// S3DiskSnapshotConfig mirrors the operator's BEX_DISK_SNAPSHOT_* contract.
type S3DiskSnapshotConfig struct {
	Endpoint, Bucket, Prefix, Region, AccessKey, SecretKey string
}

// NewS3DiskSnapshotLister returns nil when the store is not configured, which
// the verbs report as unavailable rather than failing at startup — the same
// shape every other optional bex subsystem has.
func NewS3DiskSnapshotLister(cfg S3DiskSnapshotConfig) *S3DiskSnapshotLister {
	if strings.TrimSpace(cfg.Bucket) == "" || strings.TrimSpace(cfg.Endpoint) == "" {
		return nil
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	provider := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")
	client := s3.NewFromConfig(aws.Config{
		Region: region, Credentials: aws.NewCredentialsCache(provider),
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
	return &S3DiskSnapshotLister{client: client, bucket: cfg.Bucket, prefix: strings.Trim(cfg.Prefix, "/")}
}

func (l *S3DiskSnapshotLister) ListSnapshots(ctx context.Context, prefix string) ([]DiskSnapshotObject, error) {
	full := prefix
	if l.prefix != "" {
		full = l.prefix + "/" + prefix
	}
	var out []DiskSnapshotObject
	pages := s3.NewListObjectsV2Paginator(l.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(l.bucket), Prefix: aws.String(full),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Contents {
			key := strings.TrimPrefix(aws.ToString(item.Key), l.prefix)
			out = append(out, DiskSnapshotObject{
				Key: strings.TrimPrefix(key, "/"), CreatedAt: aws.ToTime(item.LastModified),
			})
		}
	}
	return out, nil
}
