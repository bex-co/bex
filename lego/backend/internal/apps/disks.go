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
	"slices"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Persistent service disks (docs/ADR082-persistent-disks.md D6).
//
// The control-plane row is the source of truth: it is what the projector turns
// into spec.disk and what the provisioned-capacity meter bills from. Every
// refusal here mirrors a CRD-level CEL rule, so the API's 400 and the API
// server's rejection say the same thing — a Blueprint apply that bypasses
// bex-api cannot land a disk this layer would have refused.

// diskDefaultSizeGB is Render's Blueprint default; diskMaxSizeGB is the
// Hetzner Cloud volume ceiling, stated rather than left undocumented.
const (
	// diskFreeTier is the plan a disk may not be attached to: the free tier
	// sleeps and is not billable, and Render restricts disks to paid services.
	diskFreeTier      = "free"
	diskDefaultSizeGB = 10
	diskMinSizeGB     = 1
	diskMaxSizeGB     = 10000
)

// diskReservedMountPaths are the exact paths a volume must not be mounted over,
// because the platform populates them: mounting there shadows the build output,
// the projected credentials, or the whole root filesystem. Render's own list,
// plus bex's secret mount. Subdirectories of each remain legal.
var diskReservedMountPaths = []string{
	"/etc", "/etc/secrets",
	"/home", "/home/render",
	"/opt", "/opt/render", "/opt/render/project", "/opt/render/project/src",
}

// DiskView is the disk as every surface presents it, in Render's field shape.
type DiskView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MountPath string    `json:"mountPath"`
	SizeGB    int32     `json:"sizeGB"`
	ServiceID string    `json:"serviceId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func diskView(d store.Disk) DiskView {
	return DiskView{
		ID: d.ID, Name: d.Name, MountPath: d.MountPath, SizeGB: d.SizeGB,
		ServiceID: d.AppID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// AddDisk attaches a disk to a service and redeploys it. Render's
// POST /v1/disks: "The service must be redeployed for the disk to be attached",
// which here is the spec change itself — the new volume rewrites the pod
// template, so the operator rolls it (with a brief interruption, since a
// disk-bearing service deploys stop-then-start).
func (s *Service) AddDisk(ctx context.Context, serviceID, name, mountPath string, sizeGB int32) (DiskView, error) {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, serviceID)
	if err != nil {
		return DiskView{}, err
	}
	if s.Store == nil {
		return DiskView{}, errDisksUnavailable()
	}
	tenantID := a.Labels[core.LabelTenant]
	// A disk is a paid resource that starts billing the moment it exists, so it
	// goes through the same payment gate a plan change does rather than the
	// weaker mutation gate.
	if err := s.RequirePlanBilling(ctx, tenantID, a.Spec.Tier); err != nil {
		return DiskView{}, err
	}
	name, mountPath, sizeGB, err = validateDisk(a, name, mountPath, sizeGB)
	if err != nil {
		return DiskView{}, err
	}
	appID := managedAppID(a)
	if appID == "" {
		return DiskView{}, fmt.Errorf("%w: this service is not managed by the control plane", core.ErrBadRequest)
	}
	if a.Spec.Disk != nil {
		return DiskView{}, core.NewConflictError("DISK_LIMIT",
			"this service already has a disk; a service can have at most one", nil)
	}
	disk, err := s.Store.CreateDisk(ctx, tenantID, appID, name, mountPath, sizeGB)
	if err != nil {
		return DiskView{}, store.MapError(err)
	}
	if _, err := s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Disk = &appv1alpha1.DiskSpec{Name: name, MountPath: mountPath, SizeGB: sizeGB}
	}); err != nil {
		// The row is the source of truth and the projector converges the CR on
		// its next pass, so a failed patch is not a lost disk — but the caller
		// still needs to know the rollout has not happened yet.
		return DiskView{}, fmt.Errorf("attach disk: %w", err)
	}
	return diskView(disk), nil
}

// ListDisks returns the workspace's disks, or one service's when serviceID is
// set (Render's serviceIds filter, narrowed to the single-disk reality).
func (s *Service) ListDisks(ctx context.Context, serviceID string) ([]DiskView, error) {
	if serviceID != "" {
		a, err := s.AuthorizeApp(ctx, core.RelCanView, serviceID)
		if err != nil {
			return nil, err
		}
		if s.Store == nil {
			return nil, errDisksUnavailable()
		}
		disks, err := s.Store.ListDisks(ctx, a.Labels[core.LabelTenant], managedAppID(a))
		if err != nil {
			return nil, err
		}
		return diskViews(disks), nil
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, errDisksUnavailable()
	}
	disks, err := s.Store.ListDisks(ctx, s.WorkspaceOrDefault(ctx), "")
	if err != nil {
		return nil, err
	}
	return diskViews(disks), nil
}

func diskViews(disks []store.Disk) []DiskView {
	out := make([]DiskView, 0, len(disks))
	for _, d := range disks {
		out = append(out, diskView(d))
	}
	return out
}

// GetDisk reads one disk. Like every disk verb it authorizes against the
// OWNING SERVICE's workspace, never the caller's default one, so a disk id
// from another workspace is refused rather than read.
func (s *Service) GetDisk(ctx context.Context, diskID string) (DiskView, error) {
	disk, _, err := s.authorizeDisk(ctx, core.RelCanView, diskID)
	if err != nil {
		return DiskView{}, err
	}
	return diskView(disk), nil
}

// UpdateDisk applies Render's mutable fields: name, mountPath, and a GROW of
// sizeGB. A shrink is refused here, in the store, and by the CRD — three layers
// for one rule, because shrinking a filesystem under a running service destroys
// data.
func (s *Service) UpdateDisk(ctx context.Context, diskID string, name, mountPath *string, sizeGB *int32) (DiskView, error) {
	disk, a, err := s.authorizeDisk(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, diskID)
	if err != nil {
		return DiskView{}, err
	}
	if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		return DiskView{}, err
	}
	if name, err = trimAndValidate(name, validateDiskName); err != nil {
		return DiskView{}, err
	}
	if mountPath, err = trimAndValidate(mountPath, validateDiskMountPath); err != nil {
		return DiskView{}, err
	}
	if sizeGB != nil {
		if err := validateDiskSize(*sizeGB); err != nil {
			return DiskView{}, err
		}
		if *sizeGB < disk.SizeGB {
			return DiskView{}, fmt.Errorf("%w: sizeGB can only grow (current %dGB, requested %dGB)",
				core.ErrBadRequest, disk.SizeGB, *sizeGB)
		}
	}
	updated, err := s.Store.UpdateDisk(ctx, diskID, name, mountPath, sizeGB)
	if err != nil {
		return DiskView{}, store.MapError(err)
	}
	if _, err := s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Disk = &appv1alpha1.DiskSpec{
			Name: updated.Name, MountPath: updated.MountPath, SizeGB: updated.SizeGB,
		}
	}); err != nil {
		return DiskView{}, fmt.Errorf("update disk: %w", err)
	}
	return diskView(updated), nil
}

// DeleteDisk detaches and destroys the volume. Render is blunt about this and
// so is bex: "All data on the disk will be lost. The disk's associated service
// will immediately lose access to it."
func (s *Service) DeleteDisk(ctx context.Context, diskID string) error {
	_, a, err := s.authorizeDisk(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, diskID)
	if err != nil {
		return err
	}
	if err := s.Store.DeleteDisk(ctx, diskID); err != nil {
		return store.MapError(err)
	}
	if _, err := s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Disk = nil
	}); err != nil {
		return fmt.Errorf("detach disk: %w", err)
	}
	return nil
}

// authorizeDisk resolves a disk id to its row and its owning App, authorizing
// against that App. The row lookup happens first because the disk id alone
// carries no workspace — but nothing is returned to a caller that fails the
// gate, so it cannot be used to probe for a disk's existence.
func (s *Service) authorizeDisk(ctx context.Context, relation, diskID string) (store.Disk, *appv1alpha1.App, error) {
	if s.Store == nil {
		// Without a store no disk can exist, but an unauthorized caller must
		// still be refused — and audited — rather than told which subsystems
		// this deployment runs. This coarse gate exists only for that case; the
		// authoritative one below runs whenever a disk can actually be resolved,
		// so production never double-gates.
		if err := s.Authorize(ctx, relation); err != nil {
			return store.Disk{}, nil, err
		}
		return store.Disk{}, nil, errDisksUnavailable()
	}
	disk, err := s.Store.GetDisk(ctx, diskID)
	if err != nil {
		return store.Disk{}, nil, store.MapError(err)
	}
	a, err := s.AuthorizeApp(ctx, relation, disk.AppID)
	if err != nil {
		return store.Disk{}, nil, err
	}
	return disk, a, nil
}

// trimAndValidate normalizes an optional PATCH field in place. nil stays nil:
// an omitted field must be left alone, not validated into an empty one.
func trimAndValidate(value *string, validate func(string) error) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if err := validate(trimmed); err != nil {
		return nil, err
	}
	return &trimmed, nil
}

func errDisksUnavailable() error {
	return fmt.Errorf("%w: disks require the control-plane store", core.ErrUnavailable)
}

// validateDisk applies the eligibility rules the CRD enforces in CEL, so every
// surface refuses the same disk for the same stated reason instead of leaving
// it to the API server's less legible message.
func validateDisk(a *appv1alpha1.App, name, mountPath string, sizeGB int32) (string, string, int32, error) {
	name, mountPath = strings.TrimSpace(name), strings.TrimSpace(mountPath)
	if sizeGB == 0 {
		sizeGB = diskDefaultSizeGB
	}
	serviceType := a.Spec.Type
	if serviceType == "" {
		serviceType = appv1alpha1.TypeWebService
	}
	switch serviceType {
	case appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker:
	default:
		return "", "", 0, fmt.Errorf("%w: a %s cannot have a disk; disks are supported on web services, private services and background workers",
			core.ErrBadRequest, serviceType)
	}
	if a.Spec.Tier == "" || a.Spec.Tier == diskFreeTier {
		return "", "", 0, fmt.Errorf("%w: disks require a paid instance type", core.ErrBadRequest)
	}
	if a.Spec.Replicas > 1 {
		return "", "", 0, fmt.Errorf("%w: a service with a disk cannot run more than one instance (currently %d)",
			core.ErrBadRequest, a.Spec.Replicas)
	}
	if a.Spec.Autoscaling != nil && a.Spec.Autoscaling.Enabled {
		return "", "", 0, fmt.Errorf("%w: a service with a disk cannot use autoscaling; turn autoscaling off first",
			core.ErrBadRequest)
	}
	if err := validateDiskName(name); err != nil {
		return "", "", 0, err
	}
	if err := validateDiskMountPath(mountPath); err != nil {
		return "", "", 0, err
	}
	if err := validateDiskSize(sizeGB); err != nil {
		return "", "", 0, err
	}
	return name, mountPath, sizeGB, nil
}

func validateDiskName(name string) error {
	if name == "" || len(name) > 63 {
		return fmt.Errorf("%w: disk name must be 1-63 characters", core.ErrBadRequest)
	}
	return nil
}

func validateDiskMountPath(path string) error {
	switch {
	case path == "":
		return fmt.Errorf("%w: mountPath is required", core.ErrBadRequest)
	case !strings.HasPrefix(path, "/"):
		return fmt.Errorf("%w: mountPath must be an absolute path", core.ErrBadRequest)
	case len(path) > 255:
		return fmt.Errorf("%w: mountPath must be at most 255 characters", core.ErrBadRequest)
	case strings.HasSuffix(path, "/"):
		return fmt.Errorf("%w: mountPath must not be the root directory or end with a slash", core.ErrBadRequest)
	case slices.Contains(diskReservedMountPaths, path):
		return fmt.Errorf("%w: mountPath %q is reserved by the platform; mount a subdirectory of it instead",
			core.ErrBadRequest, path)
	}
	return nil
}

func validateDiskSize(sizeGB int32) error {
	if sizeGB < diskMinSizeGB || sizeGB > diskMaxSizeGB {
		return fmt.Errorf("%w: sizeGB must be between %d and %d", core.ErrBadRequest, diskMinSizeGB, diskMaxSizeGB)
	}
	return nil
}

// --- REST (Render's /v1/disks shapes) ---

// registerDiskRoutes mounts Render's disk surface. The path parameter is
// {diskId}, matching the pinned Render OpenAPI contract, so a Render client's
// generated code routes here unchanged.
func (s *Service) registerDiskRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/disks", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		disks, err := s.ListDisks(r.Context(), q.Get("serviceId"))
		if err != nil {
			return nil, err
		}
		after, limit := core.PageParams(q)
		disks = core.StablePage(disks, after, limit, q.Has("cursor") || q.Has("limit"),
			func(d DiskView) string { return d.ID })
		return toDiskList(disks), nil
	}))
	mux.HandleFunc("POST /v1/disks", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[addDiskRequest](r)
		if err != nil {
			return nil, err
		}
		return s.AddDisk(r.Context(), req.ServiceID, req.Name, req.MountPath, req.SizeGB)
	}))
	mux.HandleFunc("GET /v1/disks/{diskId}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetDisk(r.Context(), r.PathValue("diskId"))
	}))
	mux.HandleFunc("PATCH /v1/disks/{diskId}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[updateDiskRequest](r)
		if err != nil {
			return nil, err
		}
		return s.UpdateDisk(r.Context(), r.PathValue("diskId"), req.Name, req.MountPath, req.SizeGB)
	}))
	mux.HandleFunc("DELETE /v1/disks/{diskId}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteDisk(r.Context(), r.PathValue("diskId"))
	}))
}

type addDiskRequest struct {
	ServiceID string `json:"serviceId"`
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	SizeGB    int32  `json:"sizeGB"`
}

// updateDiskRequest uses pointers so an omitted field is distinguishable from
// an explicit zero — PATCH must leave what it does not name alone.
type updateDiskRequest struct {
	Name      *string `json:"name"`
	MountPath *string `json:"mountPath"`
	SizeGB    *int32  `json:"sizeGB"`
}

// toDiskList wraps disks in Render's cursor-paginated list envelope.
func toDiskList(disks []DiskView) []map[string]any {
	out := make([]map[string]any, 0, len(disks))
	for _, d := range disks {
		out = append(out, map[string]any{"disk": d, "cursor": d.ID})
	}
	return out
}

// ServiceDiskView is the disk as it appears ON A SERVICE. It carries no id:
// the service view is projected from the App CR, which knows the disk's shape
// but not the control-plane row's identity — /v1/disks is where ids live.
type ServiceDiskView struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	SizeGB    int32  `json:"sizeGB"`
}

func serviceDiskView(d *appv1alpha1.DiskSpec) *ServiceDiskView {
	if d == nil {
		return nil
	}
	return &ServiceDiskView{Name: d.Name, MountPath: d.MountPath, SizeGB: d.SizeGB}
}

// --- GraphQL ---

// diskGQLType is the /v1/disks object (with its own id); serviceDiskGQLType is
// the shape hanging off a service, which has none. Two types rather than one
// optional-id type, so a client cannot ask for an id the service view can
// never supply.
var diskGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Disk",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(d DiskView) any { return d.ID }),
		"name":      gqlutil.StrField(func(d DiskView) any { return d.Name }),
		"mountPath": gqlutil.StrField(func(d DiskView) any { return d.MountPath }),
		"sizeGB":    gqlutil.IntField(func(d DiskView) any { return d.SizeGB }),
		"serviceId": gqlutil.StrField(func(d DiskView) any { return d.ServiceID }),
		"createdAt": gqlutil.StrField(func(d DiskView) any { return d.CreatedAt.UTC().Format(time.RFC3339) }),
		"updatedAt": gqlutil.StrField(func(d DiskView) any { return d.UpdatedAt.UTC().Format(time.RFC3339) }),
	},
})

var serviceDiskGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceDisk",
	Fields: graphql.Fields{
		"name":      gqlutil.StrField(func(d ServiceDiskView) any { return d.Name }),
		"mountPath": gqlutil.StrField(func(d ServiceDiskView) any { return d.MountPath }),
		"sizeGB":    gqlutil.IntField(func(d ServiceDiskView) any { return d.SizeGB }),
	},
})

// diskGQLFields are merged into the root query/mutation by graphql.go.
func (s *Service) diskGQLQueryFields() graphql.Fields {
	return graphql.Fields{
		"disks": &graphql.Field{
			Type: graphql.NewList(diskGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				serviceID, _ := p.Args["serviceId"].(string)
				return s.ListDisks(p.Context, serviceID)
			},
		},
		"disk": &graphql.Field{
			Type: diskGQLType,
			Args: graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetDisk(p.Context, p.Args["id"].(string))
			},
		},
	}
}

func (s *Service) diskGQLMutationFields() graphql.Fields {
	return graphql.Fields{
		"addDisk": &graphql.Field{
			Type: diskGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"name":      gqlutil.ReqArg(graphql.String),
				"mountPath": gqlutil.ReqArg(graphql.String),
				"sizeGB":    &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				sizeGB, _ := p.Args["sizeGB"].(int)
				return s.AddDisk(p.Context, p.Args["serviceId"].(string),
					p.Args["name"].(string), p.Args["mountPath"].(string), int32(sizeGB))
			},
		},
		"updateDisk": &graphql.Field{
			Type: diskGQLType,
			Args: graphql.FieldConfigArgument{
				"id":        gqlutil.ReqArg(graphql.String),
				"name":      &graphql.ArgumentConfig{Type: graphql.String},
				"mountPath": &graphql.ArgumentConfig{Type: graphql.String},
				"sizeGB":    &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				var name, mountPath *string
				var sizeGB *int32
				if v, ok := p.Args["name"].(string); ok {
					name = &v
				}
				if v, ok := p.Args["mountPath"].(string); ok {
					mountPath = &v
				}
				if v, ok := p.Args["sizeGB"].(int); ok {
					size := int32(v)
					sizeGB = &size
				}
				return s.UpdateDisk(p.Context, p.Args["id"].(string), name, mountPath, sizeGB)
			},
		},
		"deleteDisk": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.DeleteDisk(p.Context, p.Args["id"].(string)); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
	}
}

// --- MCP ---

// The MCP SDK requires an object output schema, so list and delete results are
// wrapped rather than returning a bare array or string.
type listDisksResult struct {
	Disks []DiskView `json:"disks"`
}

type deleteDiskResult struct {
	Deleted bool `json:"deleted"`
}

type listDisksArgs struct {
	ServiceID string `json:"serviceId,omitempty" jsonschema:"filter to one service's disk"`
}

type diskIDArgs struct {
	DiskID string `json:"diskId" jsonschema:"the disk id (dsk-...)"`
}

type addDiskArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service to attach the disk to"`
	Name      string `json:"name" jsonschema:"a label for the disk"`
	MountPath string `json:"mountPath" jsonschema:"absolute path the disk is mounted at"`
	SizeGB    int32  `json:"sizeGB,omitempty" jsonschema:"size in GB (default 10, grow-only afterwards)"`
}

type updateDiskArgs struct {
	DiskID    string  `json:"diskId" jsonschema:"the disk id (dsk-...)"`
	Name      *string `json:"name,omitempty" jsonschema:"new label"`
	MountPath *string `json:"mountPath,omitempty" jsonschema:"new absolute mount path"`
	SizeGB    *int32  `json:"sizeGB,omitempty" jsonschema:"new size in GB; must be larger than the current size"`
}

func (s *Service) registerDiskTools(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_disks",
		Description: "List persistent disks in the workspace, optionally filtered to one service. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listDisksArgs) (*mcp.CallToolResult, listDisksResult, error) {
		disks, err := s.ListDisks(ctx, in.ServiceID)
		if err != nil {
			return nil, listDisksResult{}, err
		}
		return nil, listDisksResult{Disks: disks}, nil
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_disk",
		Description: "Get one persistent disk by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in diskIDArgs) (*mcp.CallToolResult, DiskView, error) {
		return diskResult(s.GetDisk(ctx, in.DiskID))
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name: "add_disk",
		Description: "Attach a persistent disk to a paid web service, private service, or background worker. " +
			"The service is redeployed, loses zero-downtime deploys, and can no longer run more than one instance.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addDiskArgs) (*mcp.CallToolResult, DiskView, error) {
		return diskResult(s.AddDisk(ctx, in.ServiceID, in.Name, in.MountPath, in.SizeGB))
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_disk",
		Description: "Rename a disk, change its mount path, or GROW it. A disk can never be shrunk.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateDiskArgs) (*mcp.CallToolResult, DiskView, error) {
		return diskResult(s.UpdateDisk(ctx, in.DiskID, in.Name, in.MountPath, in.SizeGB))
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "delete_disk",
		Description: "Detach and permanently destroy a persistent disk. All data on it is lost immediately.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in diskIDArgs) (*mcp.CallToolResult, deleteDiskResult, error) {
		if err := s.DeleteDisk(ctx, in.DiskID); err != nil {
			return nil, deleteDiskResult{}, err
		}
		return nil, deleteDiskResult{Deleted: true}, nil
	})
}

// diskResult is renderServiceResult's twin for the disk verbs.
func diskResult(d DiskView, err error) (*mcp.CallToolResult, DiskView, error) {
	if err != nil {
		return nil, DiskView{}, err
	}
	return nil, d, nil
}

// blueprintDisk converts render.yaml's `disk` block into the neutral view the
// create path takes, applying Render's documented default of 10 GB.
func blueprintDisk(d *bexDisk) *ServiceDiskView {
	if d == nil {
		return nil
	}
	size := d.SizeGB
	if size == 0 {
		size = diskDefaultSizeGB
	}
	return &ServiceDiskView{
		Name:      strings.TrimSpace(d.Name),
		MountPath: strings.TrimSpace(d.MountPath),
		SizeGB:    size,
	}
}

// validateCreateDisk applies the same eligibility rules AddDisk does, at create
// time. It exists because a Blueprint declares a disk inline with its service:
// there is no moment when the service exists without one, so the rules have to
// be enforced on the way in rather than on a later attach.
//
// The messages deliberately match AddDisk's, so a `render.yaml` refused here
// reads the same as the equivalent API call refused there.
func validateCreateDisk(svcType, tier string, replicas int32, disk *ServiceDiskView) (*appv1alpha1.DiskSpec, error) {
	if disk == nil {
		return nil, nil
	}
	probe := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: svcType, Tier: tier, Replicas: replicas}}
	name, mountPath, sizeGB, err := validateDisk(probe, disk.Name, disk.MountPath, disk.SizeGB)
	if err != nil {
		return nil, err
	}
	return &appv1alpha1.DiskSpec{Name: name, MountPath: mountPath, SizeGB: sizeGB}, nil
}

// blueprintDiskSizeGB is the provisioned size a declared disk contributes to a
// Blueprint's estimated monthly cost; 0 when the service declares none.
func blueprintDiskSizeGB(disk *ServiceDiskView) int32 {
	if disk == nil {
		return 0
	}
	if disk.SizeGB == 0 {
		return diskDefaultSizeGB
	}
	return disk.SizeGB
}
