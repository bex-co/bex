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

package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	metadataOwner         = "bex.co/owner"
	metadataWorkspace     = "bex.co/workspace"
	metadataNetworkPolicy = "bex.co/network-policy"
	// metadataEgress is the label the platform egress policies select a Render
	// sandbox by (w32/m283). It is separate from metadataNetworkPolicy because
	// sandboxes created before it, and agent sessions, also report deny-all but
	// must stay off these policies (they keep the legacy allowlist or their
	// session policy); enforcedPolicyStamp relies on that difference.
	metadataEgress        = sessionegress.EgressLabel
	metadataPlan          = "bex.co/plan"
	metadataTimeout       = "bex.co/timeout-seconds"
	metadataComputeWeight = "bex.co/compute-weight-milli"
	// metadataMemoryMiB is the allocated memory the memory meter bills;
	// a sandbox created before it was stamped falls back to its plan (memoryMiBOf).
	metadataMemoryMiB     = "bex.co/memory-mib"
	metadataTemplate      = "bex.co/template"
	metadataRegime        = "app.bex.co/regime"
	metadataSandboxRegime = "sandbox"
	// metadataPublicID carries the tenant-facing bex id (`sbx-<xid>`) of a
	// sandbox the EA verbs minted. OpenSandbox mints its own opaque substrate id
	// and accepts no client-supplied one, so the public identity rides the
	// metadata bex already stamps (persisted by the server as a pod label) —
	// which keeps OpenSandbox the single source of truth with no bex-side
	// mapping table to drift (ADR042 D4). Absent ⇒ a legacy (pre-w9/m94) or
	// agent-session sandbox, whose canonical id stays the substrate id.
	metadataPublicID     = "bex.co/sandbox-id"
	metadataAgentSession = agentsession.LabelSession
)

// KeyProvider mints/looks up a workspace's OpenSandbox tenant key (OSEP-0014):
// the value bex-api sends as OPEN-SANDBOX-API-KEY so the server scopes lifecycle
// ops to that workspace's `<ws>-sandbox` namespace (ADR042 D4, m32 t006).
// Implemented by the bex-api key store; nil => no key (dev/single-tenant), in
// which case the server must run single-tenant.
type KeyProvider interface {
	WorkspaceKey(ctx context.Context, workspaceID string) (string, error)
	// PurgeKeyLookup's never-minting read is also the by-id verbs' routing
	// probe (w4/m172). Required, so a provider cannot drop by-id routing by
	// lacking it (w5/m115).
	PurgeKeyLookup
}

// Template fixes a sandbox's image, entrypoint, and resource limits at
// registration time — bex's Create is template-based (ADR014 D2), never an
// arbitrary caller image. The OpenSandbox server requires an entrypoint when an
// image is given and resource limits when no pool is referenced (validated live).
type Template struct {
	Image      string
	Entrypoint []string
	CPU        string
	Memory     string
	// EphemeralStorage caps the sandbox's writable layer; "" keeps the
	// namespace LimitRange's default.
	EphemeralStorage string
	// Env is the template's own environment, under the caller's create env
	// (a caller's variable of the same name wins).
	Env map[string]string
	// SessionOnly marks a template only an agent session's sandbox runs: its
	// entrypoint needs the session env (model-proxy URL, session id,
	// placeholder key) that a public create cannot set, since every BEX_* name
	// is reserved, so the public create treats it as unknown (w7/063).
	SessionOnly bool
}

// SessionEgress is the network-mechanism seam used by the agent-session
// lifecycle (ADR047 D5). Implementations must install setup policy before Pod
// creation, make the setup→agent transition one-way, and leave the structural
// sandbox default-deny in place during cleanup.
type SessionEgress interface {
	PrepareSetup(ctx context.Context, namespace, sessionID, modelEndpoint string, egress sessionegress.Egress) error
	TransitionToAgent(ctx context.Context, namespace, sessionID, modelEndpoint string, egress sessionegress.Egress) error
	Delete(ctx context.Context, namespace, sessionID string) error
}

// SandboxEgress is the network-mechanism seam for a Render allow-list sandbox
// (sessionegress.SandboxManager): Apply runs before the sandbox exists, Rules
// reads the list back for reads, Delete runs after terminate.
type SandboxEgress interface {
	Apply(ctx context.Context, namespace, sandboxID string, domains []string) error
	Rules(ctx context.Context, namespace, sandboxID string) ([]string, error)
	Delete(ctx context.Context, namespace, sandboxID string) error
}

// Service is the authorized sandbox feature. It is stateless: the per-workspace
// tenant key scopes OpenSandbox's own list to the workspace, so OpenSandbox is
// the source of truth and bex keeps no sandbox table (ADR042 D4). Client nil =>
// every verb returns ErrSandboxesUnavailable (BEX_OPENSANDBOX_URL unset).
type Service struct {
	*core.Base
	Client      *Client
	Keys        KeyProvider
	Templates   map[string]Template
	DefaultPlan Plan
	// DefaultTemplate is used when a create names no template — the Render CLI's
	// `ea sandbox create` sends only a plan (no template flag exists), so an empty
	// template must resolve to a registered default rather than 400 (w3/m32 t009).
	DefaultTemplate string
	// Exec wires `render ea sandbox exec` (w3/m33); nil => the exec verb 503s.
	Exec *ExecConfig
	// Meter records durable lifecycle observations. nil keeps the pre-metering
	// path byte-identical for store-off deployments.
	Meter *Meter
	// SessionEgress is required only for agent-session lifecycle mechanics.
	// Ordinary Render
	// sandboxes retain the m35 compatibility policy when it is nil.
	SessionEgress SessionEgress
	// Region is the public Render region sandboxes run in (BEX_REGION through
	// resourcemeta.RenderRegion), which every sandbox and group reads (region).
	Region string
	// SandboxEgress installs a Render allow-list sandbox's per-sandbox policy
	// (w32/m283). allow-all and deny-all need none (static platform policies
	// select them by label). nil refuses allow-list creates by name.
	SandboxEgress SandboxEgress
	// Metrics reports /v1/sandboxes lifecycle outcomes (w5/m95). nil is a
	// working no-op, so a Service built without a registry needs no guard.
	Metrics *Metrics
	// Activity records exec and file activity for idle suspension (w2/m278).
	// nil (store off) disables idle suspension; wake-on-exec still works.
	Activity ActivityStore
	// Names holds Render sandbox names (w5/m287); nil (store off) refuses a
	// named create and lists every sandbox unnamed.
	Names NameStore
	// Executions holds Render's execution history (w32/m298); nil serves none.
	Executions ExecutionStore
	// Records keeps each sandbox's record past its end (w3/m318); nil (store
	// off) keeps none, and an ended sandbox is not found.
	Records RecordStore
	// Groups holds Render's sandbox groups and each sandbox's group (w22/m311);
	// nil (store off) serves no group route, refuses a create that names a group
	// and enforces no concurrency limit.
	Groups GroupStore
	// MaxGroups caps a workspace's groups (Render: 10; 0 = uncapped), past which a
	// group create answers 400 sandbox_group_limit_reached.
	MaxGroups int
	// Images imports Render's sandbox images and resolves a create's buildId or
	// buildName (w2/m358); nil answers the image routes 503.
	Images *Images
	// MaxGroupConcurrency is the ceiling and default of a group's concurrencyLimit
	// (Render: 1,000; 0 = no ceiling). A higher requested value is capped to it.
	MaxGroupConcurrency int
	// MaxActive caps the active sandboxes of one workspace across all its groups
	// (bex's own bound, ADR104 D11; 0 = uncapped), past which a create or resume
	// answers 429.
	MaxActive int
	// Storage reads a workspace's retained image bytes for the storage cap, and
	// MaxStorageBytes is that cap (bex's own bound, ADR104 D11; 0 = uncapped):
	// past it a suspend is refused with 429 SANDBOX_STORAGE_LIMIT, because it
	// would commit one more image. The image's size is unknown until it exists,
	// so a workspace can pass the cap by one image.
	Storage         StorageUsage
	MaxStorageBytes int64
	// UserIDs shows an execution's caller as its Render usr- id.
	UserIDs core.UserIDLookup
	// ResumePoll and ActivityHeartbeat override the wake-on-exec poll interval
	// and the open-stream activity heartbeat; zero uses the defaults.
	ResumePoll        time.Duration
	ActivityHeartbeat time.Duration
}

// CreateRequest is the caller's create input. OwnerID binds the workspace (as
// every create does); Template selects a registered image (empty ⇒ the default,
// since the Render CLI sends no template); Plan/TimeoutSeconds/NetworkPolicy
// are Render CLI create fields echoed back on the resource, and Region is a
// label-safe compatibility value the sandbox does not read back (region). A named Plan sizes
// the sandbox (planResources); an omitted one keeps the template's own size.
// TimeoutSeconds nil is an omitted timeout (see resolveTimeout), and
// IdleTimeoutSeconds nil an omitted idle timeout (see resolveIdleTimeout).
type CreateRequest struct {
	OwnerID            string
	Template           string
	Plan               Plan
	Region             string
	TimeoutSeconds     *int
	IdleTimeoutSeconds *int
	NetworkPolicy      *NetworkPolicy
	// Name is Render's sandbox name; empty means none.
	Name string
	// Env is Render's create-time `env`: injected into the sandbox, never
	// returned by a read.
	Env map[string]string
	// SandboxGroupID is the group to create in (Render's `sandboxGroupId`); empty
	// means the workspace's default group.
	SandboxGroupID string
	// BuildID or BuildName starts the sandbox from an image imported into its
	// group (Render's `--image`, w2/m358); both empty means the template image.
	BuildID   string
	BuildName string
}

func (s *Service) enabled() bool { return s.Client != nil }

// workspaceKey resolves the caller's workspace tenant key. Empty when no key
// provider is wired (single-tenant OpenSandbox). It keys off the RESOLVED
// workspace (s.Tenant — the caller's default when they named none, or the named
// one after the membership check), never the raw named value: a create with no
// ownerId must mint the key for the caller's own workspace, not an empty one.
func (s *Service) workspaceKey(ctx context.Context) (string, error) {
	if s.Keys == nil {
		return "", nil
	}
	ws, ok := s.Tenant(ctx)
	if !ok {
		// Multi-tenant OpenSandbox but the caller resolves to no workspace (store
		// off, or an unbound machine key): there is no `<ws>-sandbox` to scope to.
		return "", fmt.Errorf("%w: no workspace resolved for the sandbox tenant key", core.ErrForbidden)
	}
	return s.Keys.WorkspaceKey(ctx, ws)
}

// normalizeNetworkPolicy resolves a create's network policy in Render's
// semantics: omitted means allow-all; `type` and its deprecated `default`
// alias must agree; allow-list requires rules and every other policy refuses
// them. Allow-list domains are validated and canonicalized (sorted, protocol
// `https`).
func normalizeNetworkPolicy(policy *NetworkPolicy) (*NetworkPolicy, error) {
	if policy == nil {
		return &NetworkPolicy{Default: NetworkPolicyAllowAll}, nil
	}
	if policy.Type != "" && policy.Default != "" && policy.Type != policy.Default {
		return nil, core.NewBadRequestError(
			sessionegress.CodeSandboxPolicyInvalid,
			fmt.Sprintf("sandbox networkPolicy.type %q and its alias networkPolicy.default %q disagree", policy.Type, policy.Default),
			map[string]any{"field": "networkPolicy.type"},
		)
	}
	kind := policy.Default
	if policy.Type != "" {
		kind = policy.Type
	}
	if kind == "" {
		// Render: "Either `type` or `default` is required when `networkPolicy`
		// is provided."
		return nil, core.NewBadRequestError(sessionegress.CodeSandboxPolicyInvalid,
			"sandbox networkPolicy needs a type: "+strings.Join(supportedNetworkPolicies, ", "),
			map[string]any{"field": "networkPolicy.type", "supported": supportedNetworkPolicies})
	}
	switch kind {
	case NetworkPolicyAllowAll, NetworkPolicyDenyAll:
		if policy.Rules != nil {
			return nil, core.NewBadRequestError(sessionegress.CodeSandboxPolicyInvalid,
				fmt.Sprintf("sandbox networkPolicy.rules are only accepted with %q", NetworkPolicyAllowList),
				map[string]any{"field": "networkPolicy.rules"})
		}
		return &NetworkPolicy{Default: kind}, nil
	case NetworkPolicyAllowList:
		domains := make([]string, 0, len(policy.Rules))
		for _, rule := range policy.Rules {
			if rule.Protocol != "" && rule.Protocol != NetworkPolicyProtocolHTTPS {
				return nil, core.NewBadRequestError(sessionegress.CodeSandboxPolicyInvalid,
					fmt.Sprintf("sandbox networkPolicy rule protocol %q is unsupported; only %q is accepted", rule.Protocol, NetworkPolicyProtocolHTTPS),
					map[string]any{"field": "networkPolicy.rules", "reason": "protocol", "domain": rule.Domain})
			}
			domains = append(domains, rule.Domain)
		}
		canonical, err := sessionegress.SandboxAllowList(domains)
		if err != nil {
			return nil, err
		}
		return &NetworkPolicy{Default: NetworkPolicyAllowList, Rules: policyRules(canonical)}, nil
	}
	return nil, core.NewBadRequestError(sessionegress.CodeSandboxPolicyInvalid,
		fmt.Sprintf("sandbox networkPolicy.type %q is not one of %s", kind, strings.Join(supportedNetworkPolicies, ", ")),
		map[string]any{"field": "networkPolicy.type", "supported": supportedNetworkPolicies})
}

// egressCleanupTimeout bounds a best-effort policy removal that outlives its
// request, so a hung API server cannot stall a create or terminate response.
const egressCleanupTimeout = 10 * time.Second

var supportedNetworkPolicies = []string{string(NetworkPolicyAllowAll), string(NetworkPolicyDenyAll), string(NetworkPolicyAllowList)}

func policyRules(domains []string) []NetworkPolicyRule {
	rules := make([]NetworkPolicyRule, 0, len(domains))
	for _, domain := range domains {
		rules = append(rules, NetworkPolicyRule{Domain: domain, Protocol: NetworkPolicyProtocolHTTPS})
	}
	return rules
}

func ruleDomains(rules []NetworkPolicyRule) []string {
	domains := make([]string, 0, len(rules))
	for _, rule := range rules {
		domains = append(domains, rule.Domain)
	}
	return domains
}

// refuseSnapshotCreate names Render's snapshot create options, which bex does
// not serve yet, by the feature and the CLI flag rather than the wire key:
// `--snapshot-id` sends `snapshotId` and `--from` sends `snapshotName`
// (w9/067 found strict decoding answering `unknown field`, which tells a user
// nothing).
func refuseSnapshotCreate(hasSnapshot bool) error {
	if !hasSnapshot {
		return nil
	}
	return core.NewBadRequestError(
		codeSandboxSnapshotsUnsupported,
		"sandbox snapshots are not supported (--from/--snapshot-id); create a sandbox from a template instead",
		map[string]any{"field": "snapshotId", "flags": []string{"--from", "--snapshot-id"}},
	)
}

// CodeSandboxEnvInvalid refuses a create-time `env` bex cannot inject.
const CodeSandboxEnvInvalid = "SANDBOX_ENV_INVALID"

// Render's sandbox env limits (render.com/docs/sandboxes-limits, checked
// 2026-11-09): "100 variables, with a combined size of 256 KB across names
// and values".
const (
	maxSandboxEnvVars  = 100
	maxSandboxEnvBytes = 256 * 1024
)

// reservedSandboxEnvPrefixes are the platform's own variables in a sandbox:
// bex's, and those OpenSandbox gives its in-sandbox exec daemon. A caller
// value would shadow them in its own sandbox, so they are refused by name.
var reservedSandboxEnvPrefixes = []string{"BEX_", "EXECD_", "OPENSANDBOX_"}

// validateSandboxEnv applies Render's limits, the platform prefixes above, and
// a service's env rules. A
// refusal names the variable, never its value: values are often secrets.
func validateSandboxEnv(env map[string]string) error {
	if len(env) > maxSandboxEnvVars {
		return core.NewBadRequestError(CodeSandboxEnvInvalid,
			fmt.Sprintf("a sandbox takes at most %d environment variables, got %d", maxSandboxEnvVars, len(env)),
			map[string]any{"field": "env", "max": maxSandboxEnvVars})
	}
	size := 0
	// Sorted so a refusal is the same for the same input.
	for _, name := range slices.Sorted(maps.Keys(env)) {
		// A service's env rules: a Secret-key name, and a value a process
		// environment can carry.
		if err := core.CheckEnvKey(name); err != nil {
			return err
		}
		value := env[name]
		if !core.ValidEnvValue(value) {
			return core.NewBadRequestError(CodeSandboxEnvInvalid, core.EnvValueSentence(name), map[string]any{"field": "env", "name": name})
		}
		if len(name)+len("=")+len(value)+len("\x00") > core.MaxEnvEntryBytes {
			return core.NewBadRequestError(CodeSandboxEnvInvalid,
				fmt.Sprintf("environment variable %q is too large: a process cannot start with a NAME=value entry over %d bytes", name, core.MaxEnvEntryBytes-1),
				map[string]any{"field": "env", "name": name, "maxBytes": core.MaxEnvEntryBytes - 1})
		}
		upper := strings.ToUpper(name)
		for _, prefix := range reservedSandboxEnvPrefixes {
			if strings.HasPrefix(upper, prefix) {
				return core.NewBadRequestError(CodeSandboxEnvInvalid,
					fmt.Sprintf("environment variable %q is reserved for the platform (names starting with %s)", name, prefix),
					map[string]any{"field": "env", "name": name})
			}
		}
		size += len(name) + len(value)
	}
	if size > maxSandboxEnvBytes {
		return core.NewBadRequestError(CodeSandboxEnvInvalid,
			fmt.Sprintf("sandbox environment variables total %d bytes; the limit is %d across names and values", size, maxSandboxEnvBytes),
			map[string]any{"field": "env", "maxBytes": maxSandboxEnvBytes})
	}
	return nil
}

// codeSandboxSnapshotsUnsupported names Render's sandbox snapshots: the
// `snapshotId`/`snapshotName` create fields and every snapshot route.
const codeSandboxSnapshotsUnsupported = "SANDBOX_SNAPSHOTS_UNSUPPORTED"

// Render's create-sandbox refusal codes (its pinned errorCode vocabulary).
const (
	CodeInvalidSandboxName    = "invalid_sandbox_name"
	CodeInvalidSandboxPlan    = "invalid_sandbox_plan"
	CodeInvalidTimeoutSeconds = "invalid_timeout_seconds"
)

// maxSandboxName is Render's name bound, in characters.
const maxSandboxName = 64

// validateSandboxName is Render's rule: at most 64 characters and never an
// `sbx-` id, so the id stays the unambiguous handle.
func validateSandboxName(name string) error {
	if utf8.RuneCountInString(name) > maxSandboxName || strings.HasPrefix(name, ids.Sandbox.Prefix()+"-") {
		return InvalidSandboxNameError()
	}
	return nil
}

// InvalidSandboxNameError is the name refusal, shared with the REST gate,
// which refuses an over-long name before the Service sees it (w8/312).
func InvalidSandboxNameError() error {
	return core.NewBadRequestError(CodeInvalidSandboxName,
		fmt.Sprintf("sandbox name must be at most %d characters and must not start with %q", maxSandboxName, ids.Sandbox.Prefix()+"-"),
		map[string]any{"field": "name", "maxLength": maxSandboxName})
}

// invalidPlan is Render's invalid_sandbox_plan refusal: the message lists the
// accepted plans.
func invalidPlan(plan Plan) error {
	return core.NewBadRequestError(CodeInvalidSandboxPlan,
		fmt.Sprintf("unknown sandbox plan %q; use one of %s", plan, strings.Join(planNames(), ", ")),
		map[string]any{"field": "plan", "supported": planNames()})
}

func validateRegion(region string) error {
	if region != "" && len(validation.IsValidLabelValue(region)) != 0 {
		return core.NewBadRequestError(
			"SANDBOX_REGION_INVALID",
			"sandbox region must be a Kubernetes label-safe value",
			map[string]any{"field": "region"},
		)
	}
	return nil
}

// ownerID is stable and Kubernetes-label-safe because OpenSandbox persists
// request metadata as labels. Normal identity ids round-trip unchanged; an
// unusual subject is represented by a non-reversible digest instead of being
// rejected or written into metadata in an invalid form.
func ownerID(ctx context.Context) string {
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.Subject == "" {
		return "local"
	}
	if len(validation.IsValidLabelValue(id.Subject)) == 0 {
		return id.Subject
	}
	sum := sha256.Sum256([]byte(id.Subject))
	return "subject-" + hex.EncodeToString(sum[:20])
}

func sandboxMetadata(ctx context.Context, workspace, template string, plan Plan, timeout int, policy *NetworkPolicy, weight, memoryMiB int64) map[string]string {
	metadata := map[string]string{
		metadataOwner:         ownerID(ctx),
		metadataWorkspace:     workspace,
		metadataNetworkPolicy: string(policy.Default),
		metadataPlan:          string(plan),
		metadataTimeout:       timeoutMetadata(timeout),
		metadataTemplate:      template,
		metadataRegime:        metadataSandboxRegime,
		metadataComputeWeight: strconv.FormatInt(weight, 10),
		metadataMemoryMiB:     strconv.FormatInt(memoryMiB, 10),
	}
	return metadata
}

func sandboxNotFound(id string) error {
	return core.NewNotFoundError("SANDBOX_NOT_FOUND", "sandbox not found", map[string]any{"id": id})
}

// CodeSandboxCapacityLimit is the stable machine code the capacity refusal
// carries; callers that record it as a session failure (or a client keying an
// upgrade CTA) match on this rather than the message.
const CodeSandboxCapacityLimit = "SANDBOX_CAPACITY_LIMIT"

// CapacityFailureReason is the sentence an async caller records on a session
// when a create is refused for capacity. It records CodeSandboxCapacityLimit
// beside it, and a client offering an upgrade action decides by that code,
// never by this wording (w5/m132).
const CapacityFailureReason = "sandbox capacity reached"

// sandboxCapacityError is the typed refusal for the server's pod-ready-timeout
// create failure (.pm/w3/011.md fix #1). bex-api cannot see the BatchSandbox
// controller's quota condition (server + controller are external), so it names
// only what it knows — the pod did not become ready within the platform's wait —
// with workspace sandbox capacity as the likely cause and the actionable remedy,
// instead of the opaque 500 the plain upstream error mapped to. 409 matches the
// codebase's capacity-limit precedent (AGENT_SESSION_LIVE_LIMIT, ENV_GROUP_LIMIT).
func sandboxCapacityError() error {
	return core.NewConflictError(CodeSandboxCapacityLimit,
		"the sandbox did not become ready within the platform's wait; the likely cause is this workspace's sandbox capacity (plan limit) — stop an existing sandbox or upgrade the plan, then retry",
		nil)
}

// IsCapacityLimit reports whether err is (or wraps) the sandbox capacity refusal
// — the signal an async create failure uses to record CapacityFailureReason and
// CodeSandboxCapacityLimit.
func IsCapacityLimit(err error) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == CodeSandboxCapacityLimit
}

// CodeSandboxStartFailed is the code for a sandbox that did not become ready
// while the workspace still had sandbox capacity: the cause is on the platform
// side (a node without the sandbox runtime, a stuck controller), so a client
// must not offer a plan upgrade for it (w9/074).
const CodeSandboxStartFailed = "SANDBOX_START_FAILED"

// StartFailureReason is the session failure sentence recorded beside
// CodeSandboxStartFailed.
const StartFailureReason = "sandbox did not start (platform issue)"

func sandboxStartError() error {
	return core.NewUnavailableError(CodeSandboxStartFailed,
		"the sandbox did not become ready within the platform's wait, and this workspace's sandbox capacity is not exhausted, so the cause is on bex's side, not your plan; retry in a few minutes and contact support if it keeps failing",
		nil)
}

// IsStartFailure reports whether err is (or wraps) the platform-side start
// failure.
func IsStartFailure(err error) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == CodeSandboxStartFailed
}

// podReadyTimeoutError names the cause of the server's pod-ready timeout as far
// as bex-api can defend it. bex-api cannot see the pod, but it knows the
// workspace's plan limits and its live sandboxes: when they leave room for this
// sandbox, the quota did not refuse it and blaming the plan would send the
// customer to an upgrade that fixes nothing (w9/074: a node without gVisor
// failed every session as "capacity reached"). Anything bex-api cannot read
// keeps the capacity answer it gave before.
func (s *Service) podReadyTimeoutError(ctx context.Context, key, workspace, cpu, mem string) error {
	if s.capacityExhausted(ctx, key, workspace, cpu, mem) {
		return sandboxCapacityError()
	}
	return sandboxStartError()
}

func (s *Service) capacityExhausted(ctx context.Context, key, workspace, cpu, mem string) bool {
	plan, _, err := s.WorkspacePlanOf(ctx, workspace)
	if err != nil {
		log.Printf("sandbox create: pod-ready timeout: plan lookup failed (workspace=%s): %v", workspace, err)
		return true
	}
	perCPU, cpuErr := resource.ParseQuantity(cpu)
	perMem, memErr := resource.ParseQuantity(mem)
	if cpuErr != nil || memErr != nil {
		return true
	}
	live, err := s.Client.List(ctx, key)
	if err != nil {
		log.Printf("sandbox create: pod-ready timeout: list sandboxes failed (workspace=%s): %v", workspace, err)
		return true
	}
	// The timed-out sandbox may or may not be listed yet; count at least it.
	n := max(int64(len(live)), 1)
	limCPU, limMem := store.SandboxComputeLimits(plan)
	return perCPU.MilliValue()*n > limCPU.MilliValue() || perMem.Value()*n > limMem.Value()
}

// isWorkspaceAdmin answers the cross-owner admin override with an authoritative
// (uncached) decision whenever the wired checker supports core.FreshChecker
// (round-11 #5): the override gates exec-ticket minting and lifecycle mutation
// in another member's sandbox, so a just-demoted admin must not ride a positive
// cached on a different API replica for up to core.PositiveTTL. An uncached
// checker is already authoritative and degrades to plain Check. It also
// composes the relation's mapped OAuth capability (Identity.RequireCapability):
// a third-party human token carrying only bex.read fails closed to non-admin
// even when OpenFGA grants the delegated admin's can_manage. A capability
// refusal is NOT an error — callers fall back to owner-only behavior, and the
// helper stays audit-silent (no denial audit row per viewer).
func (s *Service) isWorkspaceAdmin(ctx context.Context, workspace string) (bool, error) {
	if s.Authz == nil {
		return false, nil
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok {
		return false, core.ErrForbidden
	}
	if err := id.RequireCapability(core.RelCanManage); err != nil {
		return false, nil
	}
	check := s.Authz.Check
	if fresh, ok := s.Authz.(core.FreshChecker); ok {
		check = fresh.CheckFresh
	}
	allowed, err := check(ctx, "user:"+id.Subject, core.RelCanManage, core.WorkspaceObject(workspace))
	if err != nil {
		log.Printf("sandbox: workspace authorization unavailable: %v", err)
		return false, core.ErrAuthzUnavailable
	}
	return allowed, nil
}

func validOwnedSandbox(raw osSandbox, workspace string) bool {
	return raw.ID != "" &&
		raw.Metadata[metadataOwner] != "" &&
		raw.Metadata[metadataWorkspace] == workspace &&
		raw.Metadata[metadataRegime] == metadataSandboxRegime &&
		enforcedPolicyStamp(raw.Metadata)
}

// enforcedPolicyStamp reports whether a sandbox's network-policy stamp names a
// posture the platform enforces: a w32/m283 sandbox whose egress selector
// label equals its reported policy, or a legacy deny-all sandbox (no selector
// label), which the legacy platform allowlist selects.
func enforcedPolicyStamp(metadata map[string]string) bool {
	policy, egress := metadata[metadataNetworkPolicy], metadata[metadataEgress]
	if egress == "" {
		return policy == string(NetworkPolicyDenyAll)
	}
	return policy == egress && slices.Contains(supportedNetworkPolicies, policy)
}

func (s *Service) mayAccessSandbox(ctx context.Context, workspace string, raw osSandbox) (bool, error) {
	if !validOwnedSandbox(raw, workspace) {
		return false, nil // legacy, foreign, or incompletely hardened metadata fails closed
	}
	return s.ownerCheck(ctx, workspace)(raw.Metadata[metadataOwner])
}

// ownerCheck is the caller's sandbox boundary in workspace: a sandbox the
// caller owns, or any one for a workspace admin. A list can hold many other
// members' sandboxes, so the admin check runs once, on the first of them, not
// once per sandbox.
func (s *Service) ownerCheck(ctx context.Context, workspace string) func(owner string) (bool, error) {
	caller := ownerID(ctx)
	admin, checked := false, false
	return func(owner string) (bool, error) {
		if owner == caller {
			return true, nil
		}
		if !checked {
			var err error
			if admin, err = s.isWorkspaceAdmin(ctx, workspace); err != nil {
				return false, err
			}
			checked = true
		}
		return admin, nil
	}
}

// canonicalID is the tenant-facing id of a sandbox: the stamped public
// `sbx-` id when the object carries one, else the substrate id it was minted
// with. Every surface (REST/GraphQL/MCP, metering) reads an id through here, so
// one sandbox has exactly one public identity everywhere.
//
// Agent-session sandboxes are deliberately NOT stamped: their identity is
// load-bearing INSIDE the platform — the exec gateway derives the pod name as
// `<id>-0` (internal/sandboxexec.Claims.PodName, internal/agentsession/mint.go)
// and the usage display-name join matches `agent_sessions.sandbox_id` — so
// their canonical id stays the substrate id the pod is actually named after.
// They are addressed by session id on their own surfaces, never by `sbx-`.
func canonicalID(raw osSandbox) string {
	if public := raw.Metadata[metadataPublicID]; public != "" {
		return public
	}
	return raw.ID
}

// isPublicSandboxID reports whether a caller-supplied id is a bex public
// sandbox id (rather than a substrate id), which decides how it is resolved.
func isPublicSandboxID(idStr string) bool {
	return strings.HasPrefix(idStr, ids.Sandbox.Prefix()+"-")
}

// resolveSandbox fetches one OpenSandbox object by either id form (w9/m94
// dual-accept). A substrate id is a direct Get; a public `sbx-` id is found
// through OpenSandbox's metadata filter on the tenant key's own list (the key
// scopes it to the caller's `<ws>-sandbox` namespace, so it never sees another
// tenant's object), re-checked here in case a server ignores the filter. It
// performs no authorization; ownedSandbox owns the ownership boundary.
func (s *Service) resolveSandbox(ctx context.Context, key, idStr string) (osSandbox, error) {
	if !isPublicSandboxID(idStr) {
		return s.Client.Get(ctx, key, idStr)
	}
	rows, err := s.Client.ListByMetadata(ctx, key, map[string]string{metadataPublicID: idStr})
	if err != nil {
		return osSandbox{}, err
	}
	for _, raw := range rows {
		if raw.Metadata[metadataPublicID] == idStr {
			return raw, nil
		}
	}
	return osSandbox{}, errOpenSandboxNotFound
}

// ownedSandbox resolves one durable OpenSandbox object and applies the shared
// owner/admin boundary used by reads, lifecycle verbs, and exec. Returning the
// same named not-found for absent, foreign, legacy, or incompletely hardened
// objects prevents every adapter from becoming an existence oracle.
func (s *Service) ownedSandbox(ctx context.Context, key, workspace, id string) (osSandbox, error) {
	raw, err := s.resolveSandbox(ctx, key, id)
	if err != nil {
		if errors.Is(err, errOpenSandboxNotFound) {
			return osSandbox{}, sandboxNotFound(id)
		}
		return osSandbox{}, err
	}
	allowed, err := s.mayAccessSandbox(ctx, workspace, raw)
	if err != nil {
		return osSandbox{}, err
	}
	if !allowed {
		return osSandbox{}, sandboxNotFound(id)
	}
	s.Meter.Observe(ctx, raw)
	return raw, nil
}

// sandboxView is a sandbox's public read shape. Render requires `region` on
// every sandbox, and the Render SDK refuses a response without it.
func (s *Service) sandboxView(raw osSandbox, workspace string) Sandbox {
	box := sandboxFromOpenSandbox(raw, workspace)
	box.Region = s.region()
	return box
}

// region is the one region every sandbox and sandbox group reads, so a
// sandbox and its group always agree, as on Render, where "every sandbox
// belongs to a sandbox group, which scopes it to a region" (w32/016): the
// platform's region as Render names it (BEX_REGION through
// resourcemeta.RenderRegion, as services read it), or Render's `oregon` when
// the deployment names none. A create's `region` is a compatibility value that
// does not affect placement, as the Render SDK documents it.
func (s *Service) region() string {
	if s.Region != "" {
		return s.Region
	}
	return store.SandboxGroupRegion
}

func sandboxFromOpenSandbox(raw osSandbox, workspace string) Sandbox {
	policy := &NetworkPolicy{Default: NetworkPolicyDefault(raw.Metadata[metadataNetworkPolicy])}
	if policy.Default == "" {
		policy = nil
	}
	return Sandbox{
		ID:             canonicalID(raw),
		Plan:           Plan(raw.Metadata[metadataPlan]),
		Status:         mapOpenSandboxStatus(raw.Status.State),
		TimeoutSeconds: timeoutFromMetadata(raw.Metadata[metadataTimeout]),
		// Agent-session sandboxes carry no stamp and read 0: they never
		// idle-suspend (their feature owns their lifecycle).
		IdleTimeoutSeconds: idleTimeoutFromMetadata(raw.Metadata[metadataIdleTimeout]),
		NetworkPolicy:      policy,
		Owner:              raw.Metadata[metadataOwner],
		Workspace:          raw.Metadata[metadataWorkspace],
		CreatedAt:          raw.Created,
		Image:              displayImage(raw.Image.URI),
		ungrouped:          !idleTracked(raw),
	}
}

// Create authorizes, resolves the template, and starts a sandbox scoped to the
// caller's workspace.
func (s *Service) Create(ctx context.Context, req CreateRequest) (Sandbox, error) {
	started := s.Now()
	box, err := s.create(ctx, req)
	// Measured here rather than in createResolved so agent-session sandboxes,
	// which enter through their own dispatch path, stay out of this surface's
	// signal (see Metrics).
	s.Metrics.observeCreate(err, s.Now().Sub(started))
	return box, err
}

func (s *Service) create(ctx context.Context, req CreateRequest) (Sandbox, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return Sandbox{}, err
	}
	if !s.enabled() {
		return Sandbox{}, core.ErrSandboxesUnavailable
	}
	plan := req.Plan
	if plan == "" {
		plan = s.DefaultPlan
	}
	if plan != "" && !ValidPlan(plan) {
		return Sandbox{}, invalidPlan(plan)
	}
	if err := validateRegion(req.Region); err != nil {
		return Sandbox{}, err
	}
	ws := s.WorkspaceOrDefault(ctx)
	timeout, err := resolveTimeout(req.TimeoutSeconds)
	if err != nil {
		return Sandbox{}, err
	}
	idle, err := resolveIdleTimeout(req.IdleTimeoutSeconds)
	if err != nil {
		return Sandbox{}, err
	}
	policy, err := normalizeNetworkPolicy(req.NetworkPolicy)
	if err != nil {
		return Sandbox{}, err
	}
	if err := validateSandboxName(req.Name); err != nil {
		return Sandbox{}, err
	}
	if err := validateSandboxEnv(req.Env); err != nil {
		return Sandbox{}, err
	}
	if req.Name != "" && s.Names == nil {
		return Sandbox{}, fmt.Errorf("%w: sandbox names need the control-plane store", core.ErrSandboxesUnavailable)
	}
	name := req.Template
	if name == "" {
		name = s.DefaultTemplate // Render CLI create sends no template — use the default.
	}
	tmpl, ok := s.Templates[name]
	if !ok || tmpl.SessionOnly {
		return Sandbox{}, fmt.Errorf("%w: unknown template %q", core.ErrBadRequest, name)
	}
	// A plan the caller names is the sandbox's size, as on Render; an omitted
	// plan keeps the template's registered size (the default plan only labels).
	if size, named := planResources[req.Plan]; named {
		tmpl.CPU, tmpl.Memory = size.CPU, size.Memory
	}
	// An imported image replaces the template's image and keeps its entrypoint,
	// environment and size, so the sandbox behaves as a base one does.
	image, err := s.createImage(ctx, ws, req.SandboxGroupID, req.BuildID, req.BuildName)
	if err != nil {
		return Sandbox{}, err
	}
	if image != "" {
		tmpl.Image = image
	}
	// Mint the tenant-facing id here: OpenSandbox accepts no client-supplied id,
	// so bex stamps its own and carries it as durable metadata (metadataPublicID).
	publicID := ids.New(ids.Sandbox)
	// The group's slot is taken before anything is created, under the workspace's
	// admission lock, so a group at its concurrency limit answers 429 without
	// starting a pod, and parallel creates cannot all fit into one slot.
	group, err := s.admitCreate(ctx, ws, req.SandboxGroupID, publicID)
	if err != nil {
		return Sandbox{}, err
	}
	box, err := s.launch(ctx, req, ws, publicID, name, tmpl, plan, timeout, idle, policy)
	if err != nil {
		s.releaseFailedCreate(ctx, ws, publicID, err)
		return Sandbox{}, err
	}
	s.settleSlot(ctx, ws, publicID)
	box.SandboxGroupID = group.ID
	s.recordLive(ctx, ws, box)
	return box, nil
}

// launch records the sandbox's name, then creates it under the public id.
func (s *Service) launch(ctx context.Context, req CreateRequest, ws, publicID, name string, tmpl Template, plan Plan, timeout, idle int, policy *NetworkPolicy) (Sandbox, error) {
	if req.Name != "" {
		// Written first, under the id about to be created, so a listed sandbox
		// never appears without its name; a failed create removes it.
		if err := s.Names.SetSandboxName(ctx, ws, publicID, req.Name); err != nil {
			return Sandbox{}, err
		}
		box, err := s.createWithID(ctx, ws, publicID, name, tmpl, plan, timeout, idle, policy, req.Env)
		if err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), egressCleanupTimeout)
			defer cancel()
			if cleanupErr := s.Names.DeleteSandboxName(cleanupCtx, ws, publicID); cleanupErr != nil {
				log.Printf("sandbox: remove name of failed create %s: %v", publicID, cleanupErr)
			}
			return Sandbox{}, err
		}
		box.Name = &req.Name
		return box, nil
	}
	return s.createWithID(ctx, ws, publicID, name, tmpl, plan, timeout, idle, policy, req.Env)
}

// createWithID creates the sandbox under the public id bex minted for it.
func (s *Service) createWithID(ctx context.Context, ws, publicID, name string, tmpl Template, plan Plan, timeout, idle int, policy *NetworkPolicy, env map[string]string) (Sandbox, error) {
	extra := map[string]string{
		metadataPublicID:    publicID,
		metadataIdleTimeout: strconv.Itoa(idle),
		metadataEgress:      string(policy.Default),
	}
	if policy.Default != NetworkPolicyAllowList {
		return s.createResolved(ctx, ws, name, tmpl, plan, timeout, policy, env, extra)
	}
	// An allow-list's per-sandbox policy exists before the Pod does, so the Pod
	// is never selected by anything looser than its own rules.
	if s.SandboxEgress == nil {
		return Sandbox{}, fmt.Errorf("%w: sandbox allow-list network policies are not configured", core.ErrSandboxesUnavailable)
	}
	namespace := store.SandboxNamespace(ws)
	if err := s.SandboxEgress.Apply(ctx, namespace, publicID, ruleDomains(policy.Rules)); err != nil {
		return Sandbox{}, err
	}
	box, err := s.createResolved(ctx, ws, name, tmpl, plan, timeout, policy, env, extra)
	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), egressCleanupTimeout)
		defer cancel()
		if cleanupErr := s.SandboxEgress.Delete(cleanupCtx, namespace, publicID); cleanupErr != nil {
			log.Printf("sandbox: remove allow-list policy of failed create %s: %v", publicID, cleanupErr)
		}
		return Sandbox{}, err
	}
	box.NetworkPolicy = policy
	return box, nil
}

// maxConcurrentRuleReads bounds a list's parallel policy reads against the
// Kubernetes API.
const maxConcurrentRuleReads = 8

// withRules fills an allow-list sandbox's `networkPolicy.rules` from its
// per-sandbox policy, which is where they live (labels cannot hold them).
func (s *Service) withRules(ctx context.Context, workspace string, box Sandbox) (Sandbox, error) {
	if box.NetworkPolicy == nil || box.NetworkPolicy.Default != NetworkPolicyAllowList || s.SandboxEgress == nil {
		return box, nil
	}
	domains, err := s.SandboxEgress.Rules(ctx, store.SandboxNamespace(workspace), box.ID)
	if err != nil {
		return Sandbox{}, err
	}
	policy := *box.NetworkPolicy
	policy.Rules = policyRules(domains)
	box.NetworkPolicy = &policy
	return box, nil
}

// createResolved starts a sandbox whose timeout is already resolved in
// Render's semantics (0 = no timeout; see resolveTimeout).
func (s *Service) createResolved(ctx context.Context, workspace, template string, tmpl Template, plan Plan, timeout int, policy *NetworkPolicy, env, extraMetadata map[string]string) (Sandbox, error) {
	cpu, mem := tmpl.CPU, tmpl.Memory
	if cpu == "" {
		cpu = "500m"
	}
	if mem == "" {
		mem = "512Mi"
	}
	weight, err := computeWeightMilli(cpu, mem)
	if err != nil {
		return Sandbox{}, fmt.Errorf("sandbox template resources: %w", err)
	}
	if err := s.RequirePaymentMethod(ctx, workspace); err != nil {
		return Sandbox{}, err
	}
	// codex #6: enforce the billing lifecycle gate — a delinquent/enforced
	// workspace must not start new sandbox compute, not just lack a payment method.
	if err := s.RequireBillingMutation(ctx, workspace); err != nil {
		return Sandbox{}, err
	}
	key := ""
	if s.Keys != nil {
		key, err = s.Keys.WorkspaceKey(ctx, workspace)
		if err != nil {
			return Sandbox{}, err
		}
	}
	entry := tmpl.Entrypoint
	if len(entry) == 0 {
		entry = []string{"sleep", "infinity"}
	}
	metadata := sandboxMetadata(ctx, workspace, template, plan, timeout, policy, weight, memoryMiBOf(mem))
	for k, v := range extraMetadata {
		metadata[k] = v
	}
	if len(tmpl.Env) > 0 {
		merged := maps.Clone(tmpl.Env)
		maps.Copy(merged, env)
		env = merged
	}
	limits := osResourceLimits{CPU: cpu, Memory: mem, EphemeralStorage: tmpl.EphemeralStorage}
	raw, err := s.Client.Create(ctx, key, tmpl.Image, entry, limits, openSandboxTimeout(timeout), env, metadata)
	if err != nil {
		if errors.Is(err, errPodReadyTimeout) {
			return Sandbox{}, s.podReadyTimeoutError(ctx, key, workspace, cpu, mem)
		}
		return Sandbox{}, err
	}
	if raw.Metadata == nil {
		raw.Metadata = make(map[string]string, len(metadata))
	}
	// The create response is informational; use the exact server-bound values
	// even when an OpenSandbox version omits or partially echoes metadata. Durable
	// list/get authorization still reads the separately persisted upstream copy.
	for key, value := range metadata {
		raw.Metadata[key] = value
	}
	if raw.Image.URI == "" {
		raw.Image.URI = tmpl.Image
	}
	if raw.Created == nil {
		// Render requires createdAt; an OpenSandbox create echo may omit it.
		now := s.Now()
		raw.Created = &now
	}
	s.Meter.Observe(ctx, raw)
	s.Meter.Label(ctx, raw)
	return s.sandboxView(raw, workspace), nil
}

// AgentSessionLifecycle is intentionally separate from Service's public API
// verb method set. Its caller (agentsessions.Service) owns authorization and
// audit; exposing these mechanics as sandbox verbs would create a second policy
// entry point and make the repository's verb guard correctly reject them.
type AgentSessionLifecycle struct{ service *Service }

func NewAgentSessionLifecycle(service *Service) *AgentSessionLifecycle {
	if service == nil {
		return nil
	}
	return &AgentSessionLifecycle{service: service}
}

// ModelAPIKeyEnvVar is the exact env var name the agent-image driver reads its
// BYO model provider credential from (lego/agent-image/driver/src/config.mjs,
// ADR047 D7). The tenant secret's OpenBao value must be stored under this same
// key so the fetch-to-injection path needs no translation.
const ModelAPIKeyEnvVar = agentsession.ModelKeyField

// CreateAgentSessionSandbox is the narrow trusted lifecycle seam used by the
// agent-sessions feature after it has performed the first-class session FGA
// check. It preserves every reserved sandbox hardening stamp and adds the
// durable session id, without re-checking can_create (session creation is
// intentionally can_operate per ADR047 D3).
func (l *AgentSessionLifecycle) CreateAgentSessionSandbox(ctx context.Context, workspaceID, template, sessionID, repository, branch, modelEndpoint, modelAPIKey string, egress sessionegress.Egress, driverEnv map[string]string) (Sandbox, error) {
	s := l.service
	if !s.enabled() {
		return Sandbox{}, core.ErrSandboxesUnavailable
	}
	if s.SessionEgress == nil {
		return Sandbox{}, core.ErrAgentSessionsUnavailable
	}
	if workspaceID == "" || sessionID == "" {
		return Sandbox{}, fmt.Errorf("%w: workspace and agent session id are required", core.ErrBadRequest)
	}
	if !agentsession.IsModelKeyPlaceholder(modelAPIKey, sessionID) {
		return Sandbox{}, fmt.Errorf("%w: agent session model-proxy placeholder is required", core.ErrAgentSessionsUnavailable)
	}
	if template == "" {
		template = "agent"
	}
	tmpl, ok := s.Templates[template]
	if !ok {
		return Sandbox{}, fmt.Errorf("%w: unknown agent sandbox template %q", core.ErrBadRequest, template)
	}
	// An agent session's sandbox is the agent template's own size, and its
	// plan label stays starter whatever a Render sandbox defaults to: what an
	// agent sandbox costs is ADR047's and ADR091's decision, not the sandbox
	// product's default plan (w8/247).
	plan := PlanStarter
	bindings, err := agentsession.BindingLabels(sessionID, repository, branch)
	if err != nil {
		return Sandbox{}, fmt.Errorf("%w: invalid agent session binding: %v", core.ErrBadRequest, err)
	}
	if turn := driverEnv["BEX_AGENT_TURN"]; turn != "" {
		bindings[agentsession.LabelDispatchTurn] = turn
	}
	namespace := store.SandboxNamespace(workspaceID)
	if err := s.SessionEgress.PrepareSetup(ctx, namespace, sessionID, modelEndpoint, egress); err != nil {
		return Sandbox{}, err
	}
	policy := &NetworkPolicy{Default: NetworkPolicyDenyAll}
	// The egress allowlist (a JSON array) and model endpoint (a URL) are NOT
	// stamped as sandbox metadata: OpenSandbox persists metadata as Kubernetes
	// labels, and `[]`/`https://…` are invalid label values (a `[]` empty
	// allowlist is rejected outright — caught live on prod, w3/m43). The phase
	// transition instead receives both values directly from the caller
	// (EnterAgentSessionPhase), which already holds them, so nothing needs to
	// round-trip through a label.
	// driverEnv carries the non-secret run env (prompt, agent, branch, repo,
	// delivery toggle — ADR047 D4). modelAPIKey is required to be the exact
	// session-bound proxy placeholder above; the reusable BYO key never enters a
	// sandbox Pod spec. Metadata lands in status reads and audit, so neither value
	// is stamped there.
	var env map[string]string
	if len(driverEnv) > 0 || modelAPIKey != "" {
		env = map[string]string{}
		for k, v := range driverEnv {
			env[k] = v
		}
		if modelAPIKey != "" {
			env[ModelAPIKeyEnvVar] = modelAPIKey
		}
	}
	// Setup policy belongs to the session, not this attempt. A late failure
	// must not delete a newer generation's policy; durable dispatch recovery
	// removes this attempt's sandbox independently.
	// Agent-session sandboxes keep the 86400 s bound on every plan (w5/m99).
	return s.createResolved(ctx, workspaceID, template, tmpl, plan, defaultSandboxTimeout, policy, env, bindings)
}

// EnterAgentSessionPhase narrows one exact session sandbox from setup registry
// access to its immutable agent baseline. Durable sandbox metadata, rather than
// transition-call input, supplies the model endpoint and tenant widening.
func (l *AgentSessionLifecycle) EnterAgentSessionPhase(ctx context.Context, workspaceID, sessionID, sandboxID, modelEndpoint string, egress sessionegress.Egress) error {
	s := l.service
	if s.SessionEgress == nil {
		return core.ErrAgentSessionsUnavailable
	}
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return err
	}
	// Re-validate that the sandbox is the exact one bound to this session before
	// narrowing its egress (target integrity), but take the model endpoint and
	// allowlist from the caller — they are not (and cannot be) stored as labels.
	if _, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID); err != nil {
		return err
	}
	return s.SessionEgress.TransitionToAgent(ctx, store.SandboxNamespace(workspaceID), sessionID, modelEndpoint, egress)
}

// ResumeAgentSessionSandbox resumes only a fully hardened sandbox stamped for
// this exact durable session. Workspace membership is not used as a substitute
// authorization decision here; the caller feature already checked
// agent_session:<id> in OpenFGA and this seam only enforces target integrity.
func (l *AgentSessionLifecycle) ResumeAgentSessionSandbox(ctx context.Context, workspaceID, sessionID, sandboxID string) error {
	s := l.service
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return err
	}
	raw, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID)
	if err != nil {
		return err
	}
	s.Meter.Observe(ctx, raw)
	if err := s.RequireBillingMutation(ctx, workspaceID); err != nil {
		return err
	}
	if err := s.Client.Resume(ctx, key, sandboxID); err != nil {
		return err
	}
	raw.Status.State = string(StatusResuming)
	s.Meter.Observe(ctx, raw)
	return nil
}

// CancelAgentSessionSandbox terminates the exact session sandbox. OpenSandbox's
// terminate is idempotent, so a retry can safely converge a canceling record.
func (l *AgentSessionLifecycle) CancelAgentSessionSandbox(ctx context.Context, workspaceID, sessionID, sandboxID string) error {
	s := l.service
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return err
	}
	raw, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			// Already absent is the desired terminal state. A mismatched/foreign
			// object is also deliberately indistinguishable from absent and is
			// never touched; the authorized durable session may still converge.
			if s.SessionEgress != nil {
				return s.SessionEgress.Delete(ctx, store.SandboxNamespace(workspaceID), sessionID)
			}
			return nil
		}
		return err
	}
	if err := s.terminateObserved(ctx, key, &raw); err != nil {
		return err
	}
	if s.SessionEgress != nil {
		if err := s.SessionEgress.Delete(ctx, store.SandboxNamespace(workspaceID), sessionID); err != nil {
			return fmt.Errorf("delete terminated session egress policy: %w", err)
		}
	}
	return nil
}

// terminateObserved meters the live sandbox, terminates it, then meters the
// terminal state so usage closes the compute window. Shared by cancel, dispatch
// cleanup, and inventory reconcile.
func (s *Service) terminateObserved(ctx context.Context, key string, raw *osSandbox) error {
	s.Meter.Observe(ctx, *raw)
	if err := s.Client.Terminate(ctx, key, raw.ID); err != nil {
		return err
	}
	raw.Status.State = string(StatusTerminated)
	s.Meter.Observe(ctx, *raw)
	s.recordEnded(ctx, raw.Metadata[metadataWorkspace], *raw)
	return nil
}

// agentSessionStatusPath is the driver's machine-readable status file inside the
// sandbox (BEX_AGENT_STATUS_FILE default, lego/agent-image/driver/src/config.mjs).
const agentSessionStatusPath = "/var/run/bex-agent/status.json"

// ReadSessionStatus returns the raw driver status-file contents from the exact
// session sandbox through the gateway exec boundary — the trusted completion
// signal the agent-sessions Completer polls (ADR047 D4). It re-validates the
// sandbox binding but intentionally performs no user-facing authorization: its
// only caller is a system loop that already owns the durable session. Empty
// output means the file is not present yet (the headless turn is still running).
func (l *AgentSessionLifecycle) ReadSessionStatus(ctx context.Context, workspaceID, sessionID, sandboxID string) (string, error) {
	s := l.service
	status, err := s.sessionSandboxState(ctx, workspaceID, sessionID, sandboxID)
	if err != nil {
		return "", err
	}
	// A sandbox whose pod has exited (a crashed/hung driver, a failed adapter
	// start) can never report a success status, and exec-ing into the dead pod
	// fails forever — which silently stranded the session in `running` (w3/m43
	// live E2E, the nonexistent-adapter crash leg). Surface it as NotFound so the
	// Completer finalizes it as a failed turn instead of retrying every tick.
	switch status {
	case StatusTerminated:
		return "", sandboxNotFound(sandboxID)
	case StatusErrored:
		// Its container ended. One no-op exec fails, and while the Pod is still
		// there the gateway's target check answers how (OOMKilled, an exit
		// code), so the session fails with its cause before the sandbox is
		// deleted. NotFound either way.
		_, err := s.sessionExec(ctx, workspaceID, sessionID, sandboxID, "true", maxExecOutputBytes)
		if _, ok := TerminationOf(err); ok {
			return "", err
		}
		return "", sandboxNotFound(sandboxID)
	}
	result, err := s.sessionExec(ctx, workspaceID, sessionID, sandboxID,
		"cat "+agentSessionStatusPath+" 2>/dev/null || true", maxExecOutputBytes)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

// agentSessionLogPath is the driver's per-part conversation log inside
// the sandbox (BEX_AGENT_SESSION_LOG default, lego/agent-image/driver/src/config.mjs).
const agentSessionLogPath = "/var/log/bex-agent/session.jsonl"

// maxTranscriptLogBytes bounds the exec payload when harvesting the log (the tail
// is read; the parser drops a partial leading line). A turn's transcript is far
// smaller in practice; this only guards a pathological driver.
const maxTranscriptLogBytes = 16 << 20

// ReadSessionTranscript returns the driver's session log (JSONL, one
// UI-message part per line) from the exact session sandbox through the SAME
// gateway pods/exec boundary the Completer already uses for ReadSessionStatus
// (ADR051). Harvesting the on-disk log rides the proven status-read path instead
// of a separate driver-stream dial. A terminated/errored pod (its log gone with
// it) surfaces as NotFound so the caller treats it as "nothing to capture".
func (l *AgentSessionLifecycle) ReadSessionTranscript(ctx context.Context, workspaceID, sessionID, sandboxID string) (string, error) {
	s := l.service
	status, err := s.sessionSandboxState(ctx, workspaceID, sessionID, sandboxID)
	if err != nil {
		return "", err
	}
	if status == StatusTerminated || status == StatusErrored {
		return "", sandboxNotFound(sandboxID)
	}
	// The public buffered-exec cap is 2 MiB, but the driver's own log is
	// bounded at 16 MiB. Collect that full trusted artifact with framing headroom
	// and fail explicitly if even this bound truncates it.
	result, err := s.sessionExec(ctx, workspaceID, sessionID, sandboxID,
		fmt.Sprintf("tail -c %d %s 2>/dev/null || true", maxTranscriptLogBytes, agentSessionLogPath),
		maxTranscriptLogBytes+1<<20)
	if err != nil {
		return "", err
	}
	if result.Truncated {
		return "", fmt.Errorf("%w: agent session transcript exceeded harvest limit", core.ErrSandboxesUnavailable)
	}
	return result.Stdout, nil
}

// sessionSandboxState re-validates a session read's sandbox binding and
// returns its lifecycle state. A state bex does not know is refused as
// errStateUnknown, retried and never read as a dead sandbox. Exec
// configuration is checked after a terminated state, so a sandbox past its
// lifetime still reads as gone when exec is not configured.
func (s *Service) sessionSandboxState(ctx context.Context, workspaceID, sessionID, sandboxID string) (Status, error) {
	if !s.enabled() {
		return "", core.ErrSandboxesUnavailable
	}
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	sb, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID)
	if err != nil {
		return "", err
	}
	status, known := openSandboxStatus(sb.Status.State)
	switch {
	case !known:
		return "", errStateUnknown
	case status != StatusTerminated && !s.execEnabled():
		return "", core.ErrSandboxesUnavailable
	}
	return status, nil
}

// sessionExec runs one trusted Completer command in the session sandbox
// through the gateway and buffers its output up to limit.
func (s *Service) sessionExec(ctx context.Context, workspaceID, sessionID, sandboxID, command string, limit int) (ExecResult, error) {
	return s.systemExec(ctx, workspaceID, sandboxID, sessionID, []string{"/bin/sh", "-c", command}, limit)
}

// systemExec runs one trusted argv of bex-api's own in a sandbox and buffers
// its output up to limit: no caller-facing wrapper, record or activity.
func (s *Service) systemExec(ctx context.Context, ws, sandboxID, agentSessionID string, argv []string, limit int) (ExecResult, error) {
	resp, err := s.mintAndDial(ctx, ws, sandboxID, agentSessionID, argv)
	if err != nil {
		return ExecResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	return bufferExecWithLimit(resp, limit)
}

func (s *Service) agentSessionKey(ctx context.Context, workspaceID string) (string, error) {
	if !s.enabled() {
		return "", core.ErrSandboxesUnavailable
	}
	if s.Keys == nil {
		return "", nil
	}
	return s.Keys.WorkspaceKey(ctx, workspaceID)
}

func (s *Service) agentSessionSandbox(ctx context.Context, key, workspaceID, sessionID, sandboxID string) (osSandbox, error) {
	raw, err := s.Client.Get(ctx, key, sandboxID)
	if err != nil {
		if errors.Is(err, errOpenSandboxNotFound) {
			return osSandbox{}, sandboxNotFound(sandboxID)
		}
		return osSandbox{}, err
	}
	if !validOwnedSandbox(raw, workspaceID) || raw.Metadata[metadataAgentSession] != sessionID {
		return osSandbox{}, sandboxNotFound(sandboxID)
	}
	return raw, nil
}

// List returns the caller's workspace's sandboxes (tenant-key-scoped).
func (s *Service) List(ctx context.Context) ([]Sandbox, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if !s.enabled() {
		return nil, core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return nil, err
	}
	ws := s.WorkspaceOrDefault(ctx)
	raw, err := s.Client.List(ctx, key)
	if err != nil {
		return nil, err
	}
	out := make([]Sandbox, 0, len(raw))
	visible := s.ownerCheck(ctx, ws)
	for _, r := range raw {
		if !validOwnedSandbox(r, ws) {
			continue
		}
		if ok, err := visible(r.Metadata[metadataOwner]); err != nil {
			return nil, err
		} else if ok {
			out = append(out, s.sandboxView(r, ws))
		}
	}
	// Each allow-list sandbox's rules are one policy read; run them side by
	// side so a list costs about one round trip, not one per sandbox.
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(maxConcurrentRuleReads)
	for i := range out {
		group.Go(func() error {
			box, err := s.withRules(groupCtx, ws, out[i])
			out[i] = box
			return err
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	named, err := s.withNames(ctx, ws, out)
	if err != nil {
		return nil, err
	}
	return s.withGroups(ctx, ws, named)
}

// withGroups fills each Render sandbox's group: the one it was created in, or
// the workspace's default for a sandbox with no binding (one created before
// groups existed). A workspace with such a sandbox gets its default group here
// if it has none yet. Agent-session sandboxes are not Render sandboxes and have
// no group.
func (s *Service) withGroups(ctx context.Context, workspace string, boxes []Sandbox) ([]Sandbox, error) {
	if s.Groups == nil || len(boxes) == 0 {
		return boxes, nil
	}
	members, err := s.Groups.SandboxGroupMembers(ctx, workspace)
	if err != nil {
		return nil, store.MapError(err)
	}
	var defaultID string
	for i := range boxes {
		if boxes[i].ungrouped {
			continue
		}
		if member, bound := members[boxes[i].ID]; bound {
			boxes[i].SandboxGroupID = member.GroupID
			continue
		}
		if defaultID == "" {
			def, err := s.defaultGroup(ctx, workspace)
			if err != nil {
				return nil, err
			}
			defaultID = def.ID
		}
		boxes[i].SandboxGroupID = defaultID
	}
	return boxes, nil
}

// NameStore is where Render sandbox names live; *store.PGStore implements it.
type NameStore interface {
	SetSandboxName(ctx context.Context, workspaceID, sandboxID, name string) error
	DeleteSandboxName(ctx context.Context, workspaceID, sandboxID string) error
	SandboxNames(ctx context.Context, workspaceID string) (map[string]string, error)
	PruneSandboxNames(ctx context.Context, workspaceID string, live []string) error
}

// withNames fills each sandbox's Render name.
func (s *Service) withNames(ctx context.Context, workspace string, boxes []Sandbox) ([]Sandbox, error) {
	if s.Names == nil || len(boxes) == 0 {
		return boxes, nil
	}
	names, err := s.Names.SandboxNames(ctx, workspace)
	if err != nil {
		return nil, err
	}
	for i := range boxes {
		if name, ok := names[boxes[i].ID]; ok {
			boxes[i].Name = &name
		}
	}
	return boxes, nil
}

// listFiltered is List narrowed by Render's status filter, newest first — the
// one helper the REST, GraphQL and MCP list adapters share, so all three apply
// the same default, order and refusals. The filter is validated before the
// authorized List runs.
func (s *Service) listFiltered(ctx context.Context, status []string, name, groupID string) ([]Sandbox, error) {
	want, err := parseStatusFilter(status)
	if err != nil {
		return nil, err
	}
	if groupID != "" {
		if err := validateGroupID(groupID); err != nil {
			return nil, err
		}
		if err := s.requireGroups(); err != nil {
			return nil, err
		}
	}
	out, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	if want[StatusTerminated] {
		endedBoxes, err := s.endedSandboxes(ctx, s.WorkspaceOrDefault(ctx), out)
		if err != nil {
			return nil, err
		}
		out = append(out, endedBoxes...)
	}
	out = filterStatuses(out, want)
	if groupID != "" {
		// Render: "the group must belong to ownerId". A group of another
		// workspace is the same not-found as one that does not exist.
		if _, err := s.storedGroup(ctx, groupID); err != nil {
			return nil, err
		}
		out = core.Filter(out, func(sb Sandbox) bool { return sb.SandboxGroupID == groupID })
	}
	if name != "" {
		// Render's exact-match name filter; names are not unique.
		out = core.Filter(out, func(box Sandbox) bool { return box.Name != nil && *box.Name == name })
	}
	slices.SortFunc(out, newestFirst)
	return out, nil
}

// newestFirst orders sandboxes as Render lists them (render.com/docs/sandboxes,
// read 2026-11-12: "Lists sandboxes in the specified workspace, newest
// first"). OpenSandbox lists oldest first. The id breaks a tie (ids are
// unique), so a cursor page is stable; a sandbox with no creation time sorts
// last.
func newestFirst(a, b Sandbox) int {
	switch {
	case a.CreatedAt == nil && b.CreatedAt != nil:
		return 1
	case a.CreatedAt != nil && b.CreatedAt == nil:
		return -1
	case a.CreatedAt != nil && !a.CreatedAt.Equal(*b.CreatedAt):
		return b.CreatedAt.Compare(*a.CreatedAt)
	}
	return strings.Compare(b.ID, a.ID)
}

// Get returns one sandbox's status. With no workspace named in ctx it acts in
// the sandbox's own workspace when the caller is a member there (w4/m172).
func (s *Service) Get(ctx context.Context, id string) (Sandbox, error) {
	ctx, err := s.scopeSandbox(ctx, "", id)
	if err != nil {
		return Sandbox{}, err
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Sandbox{}, err
	}
	if !s.enabled() {
		return Sandbox{}, core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return Sandbox{}, err
	}
	ws := s.WorkspaceOrDefault(ctx)
	raw, err := s.ownedSandbox(ctx, key, ws, id)
	if err != nil {
		return s.orEnded(ctx, ws, id, err)
	}
	box, err := s.withRules(ctx, ws, s.sandboxView(raw, ws))
	if err != nil {
		return Sandbox{}, err
	}
	named, err := s.withNames(ctx, ws, []Sandbox{box})
	if err != nil {
		return Sandbox{}, err
	}
	grouped, err := s.withGroups(ctx, ws, named)
	if err != nil {
		return Sandbox{}, err
	}
	return grouped[0], nil
}

// Suspend/Resume/Terminate are the lifecycle verbs; suspend/resume need operate,
// terminate needs create (delete) — matching the other resource features.
func (s *Service) Suspend(ctx context.Context, id string) error {
	return s.lifecycle(ctx, core.RelCanOperate, id, StatusSuspended, s.clientSuspend)
}
func (s *Service) Resume(ctx context.Context, id string) error {
	return s.lifecycle(ctx, core.RelCanOperate, id, StatusResuming, s.clientResume)
}
func (s *Service) Terminate(ctx context.Context, id string) error {
	err := s.lifecycle(ctx, core.RelCanCreate, id, StatusTerminated, s.clientTerminate)
	s.Metrics.observeTerminate(err)
	return err
}

func (s *Service) clientSuspend(ctx context.Context, key string, raw osSandbox) error {
	return s.Client.Suspend(ctx, key, raw.ID)
}
func (s *Service) clientResume(ctx context.Context, key string, raw osSandbox) error {
	return s.Client.Resume(ctx, key, raw.ID)
}
func (s *Service) clientTerminate(ctx context.Context, key string, raw osSandbox) error {
	if err := s.Client.Terminate(ctx, key, raw.ID); err != nil {
		return err
	}
	// Render finalizes a terminated sandbox's open executions.
	if s.Executions != nil {
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), executionStopTimeout)
		defer cancel()
		if err := s.Executions.StopOpenSandboxExecutions(stopCtx, raw.Metadata[metadataWorkspace], canonicalID(raw), s.Now()); err != nil {
			log.Printf("sandbox: stop executions of terminated %s: %v", canonicalID(raw), err)
		}
	}
	// A terminated allow-list sandbox's policy selects nothing any more; remove
	// it so the namespace does not accumulate them. Failure only leaks an inert
	// object, so it never fails the terminate.
	if s.SandboxEgress != nil && raw.Metadata[metadataEgress] == string(NetworkPolicyAllowList) {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), egressCleanupTimeout)
		defer cancel()
		if err := s.SandboxEgress.Delete(cleanupCtx, store.SandboxNamespace(raw.Metadata[metadataWorkspace]), canonicalID(raw)); err != nil {
			log.Printf("sandbox: remove allow-list policy of terminated %s: %v", canonicalID(raw), err)
		}
	}
	return nil
}

// endTerminated is what a terminate does after OpenSandbox accepted it, for a
// sandbox ended without a caller's lifecycle verb (its group was deleted,
// w2/m359): its slot, its meter and its record.
func (s *Service) endTerminated(ctx context.Context, workspace string, raw osSandbox) {
	s.releaseSlot(ctx, workspace, canonicalID(raw))
	raw.Status.State = string(StatusTerminated)
	s.Meter.Observe(ctx, raw)
	s.recordEnded(ctx, workspace, raw)
}

func (s *Service) lifecycle(ctx context.Context, relation, id string, phase Status, op func(context.Context, string, osSandbox) error) error {
	ctx, err := s.scopeSandbox(ctx, "", id)
	if err != nil {
		return err
	}
	if err := s.Authorize(ctx, relation); err != nil {
		return err
	}
	if !s.enabled() {
		return core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return err
	}
	ws := s.WorkspaceOrDefault(ctx)
	raw, err := s.ownedSandbox(ctx, key, ws, id)
	if err != nil {
		box, endedErr := s.orEnded(ctx, ws, id, err)
		switch {
		case endedErr != nil:
			return endedErr
		case phase == StatusTerminated:
			return nil // Render: terminate is idempotent, 204 from any state
		default:
			return sandboxEndedError(box)
		}
	}
	undoAdmission := func() {}
	if phase == StatusResuming {
		// Generic retained sandboxes and agent-session sandboxes share this
		// trusted seam; the billing decision must be made immediately before the
		// provider resume, not only in an outer REST/GraphQL adapter.
		if err := s.RequireBillingMutation(ctx, ws); err != nil {
			return err
		}
		// A resume needs a free slot in the sandbox's group, like the implicit one
		// an exec starts (resumeForActivity).
		if undoAdmission, err = s.admitResume(ctx, key, ws, raw); err != nil {
			return err
		}
		if err := s.beforeResume(ctx, key, raw); err != nil {
			undoAdmission()
			return err
		}
	}
	if phase == StatusSuspended {
		if err := s.beforeSuspend(ctx, key, raw); err != nil {
			return err
		}
	}
	if err := op(ctx, key, raw); err != nil {
		undoAdmission()
		switch {
		case errors.Is(err, errOpenSandboxNotFound):
			return sandboxNotFound(id)
		case errors.Is(err, errOpenSandboxConflict):
			return core.NewConflictError("SANDBOX_LIFECYCLE_CONFLICT",
				fmt.Sprintf("sandbox %s is %s; wait for it to settle and retry", id, mapOpenSandboxStatus(raw.Status.State)),
				map[string]any{"id": id, "status": string(mapOpenSandboxStatus(raw.Status.State))})
		}
		return err
	}
	if phase == StatusResuming && idleTracked(raw) {
		// A resumed sandbox restarts its runtime timeout and its idle window,
		// and any idle-sweeper claim on it is spent.
		s.afterResume(ctx, key, raw)
		s.touchActivity(ctx, ws, raw.ID)
		s.clearSuspendClaim(ctx, ws, raw.ID)
	}
	if idleTracked(raw) {
		switch phase {
		case StatusSuspended:
			s.afterSuspend(ctx, raw)
			// A suspended sandbox holds no slot.
			s.clearSlotLease(ctx, ws, canonicalID(raw))
		case StatusTerminated:
			s.releaseSlot(ctx, ws, canonicalID(raw))
		}
	}
	raw.Status.State = string(phase)
	s.Meter.Observe(ctx, raw)
	if phase == StatusTerminated {
		s.recordEnded(ctx, ws, raw)
	}
	if phase == StatusTerminated && s.SessionEgress != nil {
		if sessionID := raw.Metadata[metadataAgentSession]; sessionID != "" {
			if err := s.SessionEgress.Delete(ctx, store.SandboxNamespace(ws), sessionID); err != nil {
				return fmt.Errorf("delete terminated session egress policy: %w", err)
			}
		}
	}
	return nil
}
