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

package pricing

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w7/m429 t005: every row of Render's two plan tables — the comparison table
// on render.com/pricing and render.com/docs/platform-features-by-plan, both
// captured in testdata/ on 2026-11-16 and re-read on 2026-11-26 — maps to exactly one recorded
// disposition in bex. A row Render adds, or one bex forgets, fails here; so
// does a disposition whose row is gone. Where the disposition is a number
// (members, services, environments, kept builds, included domains, minutes and
// bandwidth, retention days), the number Render shows is compared with the
// code's. ADR030 §9 lists the same dispositions.

var renderPlans = []string{store.PlanHobby, store.PlanPro, store.PlanScale, store.PlanEnterprise}

// planRow is one row of a captured table: values is one cell per plan in
// renderPlans order (the comparison table), or the lowest plan that includes
// the feature (features by plan).
type planRow struct {
	key    string // "<table>: <row title>"
	values []string
}

// reporter is the part of *testing.T a disposition's check reports through,
// so the guard's own test can watch a check fail.
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// failures is a reporter that records what failed.
type failures struct {
	failed   bool
	messages []string
}

func (*failures) Helper() {}
func (f *failures) Errorf(format string, args ...any) {
	f.failed = true
	f.messages = append(f.messages, fmt.Sprintf(format, args...))
}

// planDisposition is how bex handles one row.
type planDisposition struct {
	// kind is one of: same (identical on every plan, in Render and in bex),
	// gate (a core.PlanFeature), limit, meter (included then metered),
	// retention, nonGoal (DO_NOT_DO or an ADR018 non-goal row), gap (an open
	// ADR018 gap or board item), followUp (a plan difference not yet applied,
	// filed on the board), commercial (a contract or support term, not a
	// product surface).
	kind string
	// ref is where bex records it.
	ref string
	// check compares the row's values with the code's, when the disposition
	// carries a rule or a number.
	check func(t reporter, row planRow)
}

func same(ref string) planDisposition {
	return planDisposition{kind: "same", ref: ref, check: func(t reporter, row planRow) {
		t.Helper()
		for _, v := range row.values {
			if v != row.values[0] {
				t.Errorf("%s: Render now varies it by plan (%q); bex treats it as the same on every plan (%s)", row.key, row.values, ref)
			}
		}
	}}
}

// gate checks the comparison table's cells against core.PlanAllows: a blank
// cell, or one that keeps the feature to datastores, means the plan lacks it.
func gate(feature core.PlanFeature) planDisposition {
	return planDisposition{kind: "gate", ref: "core.planFeatures " + string(feature), check: func(t reporter, row planRow) {
		t.Helper()
		for i, v := range row.values {
			offered := v != "-" && !strings.HasPrefix(v, "Datastores only")
			if got := core.PlanAllows(renderPlans[i], feature); got != offered {
				t.Errorf("%s on %s: Render %q, bex PlanAllows = %v", row.key, renderPlans[i], v, got)
			}
		}
	}}
}

// gateBoth is gate for a row that names two settings bex gates apart (w7/m465:
// "Enforce 2FA & Google login"): each must follow the row.
func gateBoth(first, second core.PlanFeature) planDisposition {
	a, b := gate(first), gate(second)
	return planDisposition{kind: "gate", ref: a.ref + "; " + b.ref, check: func(t reporter, row planRow) {
		t.Helper()
		a.check(t, row)
		b.check(t, row)
	}}
}

// minimumGate checks a features-by-plan row's lowest plan against
// core.PlanAllows.
func minimumGate(feature core.PlanFeature) planDisposition {
	return planDisposition{kind: "gate", ref: "core.planFeatures " + string(feature), check: func(t reporter, row planRow) {
		t.Helper()
		minimum := slices.Index(renderPlans, row.values[0])
		for i, plan := range renderPlans {
			if got := core.PlanAllows(plan, feature); got != (i >= minimum) {
				t.Errorf("%s: Render's lowest plan is %s, bex PlanAllows(%s) = %v", row.key, row.values[0], plan, got)
			}
		}
	}}
}

// numbered checks each plan's cell against the code's number for it. parse
// reads a cell (-1 for unlimited, ok false for a cell bex does not bind, like
// Enterprise's "Custom"); value is the code's number (-1 for unlimited).
func numbered(kind, ref string, parse func(cell string) (n int64, ok bool), value func(plan string) int64) planDisposition {
	return planDisposition{kind: kind, ref: ref, check: func(t reporter, row planRow) {
		t.Helper()
		for i, cell := range row.values {
			want, ok := parse(cell)
			if !ok {
				continue
			}
			if got := value(renderPlans[i]); got != want {
				t.Errorf("%s on %s: Render %q (%d), bex %d (%s)", row.key, renderPlans[i], cell, want, got, ref)
			}
		}
	}}
}

// leadingNumber reads a cell's first number, with Render's K/TB shorthand
// ("1K mins", "1 TB" = 1000 GB); "Unlimited" is -1 and "Custom" is unbound.
func leadingNumber(cell string) (int64, bool) {
	switch {
	case cell == "Unlimited":
		return -1, true
	case cell == "Custom":
		return 0, false
	}
	field := strings.Fields(cell)[0]
	scale := int64(1)
	if digits, ok := strings.CutSuffix(field, "K"); ok {
		field, scale = digits, 1000
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(field, ",", ""), 10, 64)
	if err != nil {
		return 0, false
	}
	if strings.Contains(cell, " TB ") {
		scale = 1000
	}
	return n * scale, true
}

// unlimitedAtZero turns a "0 = unlimited" cap into the table's -1.
func unlimitedAtZero(n int) int64 {
	if n == 0 {
		return -1
	}
	return int64(n)
}

func nonGoal(ref string) planDisposition    { return planDisposition{kind: "nonGoal", ref: ref} }
func gap(ref string) planDisposition        { return planDisposition{kind: "gap", ref: ref} }
func followUp(ref string) planDisposition   { return planDisposition{kind: "followUp", ref: ref} }
func commercial(ref string) planDisposition { return planDisposition{kind: "commercial", ref: ref} }

// increased checks a features-by-plan "Increased …" line: bex's number is
// larger on Pro than on Hobby, and no smaller above Pro.
func increased(ref string, value func(plan string) int64) planDisposition {
	return planDisposition{kind: "limit", ref: ref, check: func(t reporter, row planRow) {
		t.Helper()
		if value(store.PlanPro) <= value(store.PlanHobby) {
			t.Errorf("%s: bex's Pro (%d) is not above Hobby (%d)", row.key, value(store.PlanPro), value(store.PlanHobby))
		}
		for _, plan := range renderPlans[2:] {
			if value(plan) < value(store.PlanPro) {
				t.Errorf("%s: bex's %s (%d) is below Pro (%d)", row.key, plan, value(plan), value(store.PlanPro))
			}
		}
	}}
}

func keptBuilds(plan string) int64 {
	n, _ := core.KeptBuilds(plan)
	return int64(n)
}

func concurrentSandboxes(plan string) int64 {
	return int64(core.ConcurrentSandboxes(plan))
}

func includedDomains(plan string) int64 {
	n, unlimited := core.IncludedCustomDomains(plan)
	if unlimited {
		return -1
	}
	return int64(n)
}

func includedBuildMinutes(plan string) int64 {
	return Default.IncludedFor(plan)[store.UsageKindBuildSeconds] / 60
}

func includedBandwidthGB(plan string) int64 {
	return Default.IncludedFor(plan)[store.UsageKindEgressBytes] / BytesPerGiB
}

func logRetentionDays(plan string) int64 {
	return int64(core.LogRetention(plan) / (24 * time.Hour))
}

func metricsRetentionDays(plan string) int64 {
	return int64(core.MetricsRetention(plan) / (24 * time.Hour))
}

func pitrRetentionDays(plan string) int64 {
	return int64(core.PITRRetention(plan) / (24 * time.Hour))
}

// The board item a followUp names for rows bex does not apply yet, filed by
// w7/m429 t005.
const followUpHIPAALinks = "w7/071 (HIPAA-enabled workspaces and AWS private links have no recorded disposition)"

// The ADR018 row and board item a gap names for Render's compliance documents,
// which Render's dashboard serves on its Compliance and Documents page (w4/746).
const gapComplianceDocuments = "ADR018 Compliance and Documents page, w4/746 (the user decides which documents bex offers)"

// planDispositions is bex's answer to every captured row.
var planDispositions = map[string]planDisposition{
	// render.com/pricing — Render Services & Storage
	"pricing: Maximum services":              numbered("limit", "store.LimitsFor MaxServices", leadingNumber, func(p string) int64 { return unlimitedAtZero(store.LimitsFor(p).MaxServices) }),
	"pricing: Static Sites":                  same("ADR029"),
	"pricing: Standard services":             same("ADR018 Services"),
	"pricing: Web services":                  same("ADR018 Services"),
	"pricing: Private services":              same("ADR018 Services"),
	"pricing: Background workers":            same("ADR018 Services"),
	"pricing: Persistent disks":              same("ADR082, pricing.yaml disk"),
	"pricing: Postgres databases":            same("ADR009"),
	"pricing: Expandable storage":            same("ADR030 §2, pricing.yaml storage"),
	"pricing: Point-in-time recovery window": numbered("retention", "core.pitrRetention (w7/m453)", leadingNumber, pitrRetentionDays),
	"pricing: Read-only replicas":            same("ADR009"),
	"pricing: High availability":             same("ADR009"),
	"pricing: Key-value (Redis-compatible)":  same("ADR021"),
	"pricing: Cron jobs":                     same("ADR018 Cron jobs"),
	"pricing: Workflows":                     nonGoal("DO_NOT_DO: Workflows & tasks; ADR018 Workflows & tasks"),
	"pricing: Parallel task compute":         nonGoal("DO_NOT_DO: Workflows & tasks"),
	"pricing: New task compute per minute":   nonGoal("DO_NOT_DO: Workflows & tasks"),
	"pricing: Sandboxes":                     same("ADR104"),
	"pricing: Default concurrent sandboxes":  numbered("limit", "core.ConcurrentSandboxes", leadingNumber, concurrentSandboxes),
	"pricing: Maximum session length":        same("ADR104"),
	"pricing: Snapshot storage":              same("ADR030 §15"),
	"pricing: Service scaling":               same("ADR018 Scaling"),
	"pricing: Vertical scaling":              same("ADR018 Instance types"),
	"pricing: Horizontal autoscaling":        gate(core.FeatureAutoscaling),
	// Networking
	"pricing: Automatic private networking": same("ADR043"),
	"pricing: Bandwidth": numbered("meter", "ADR030 §14, pricing.yaml bandwidth.includedGB",
		leadingNumber, includedBandwidthGB),
	"pricing: Global regions":                  same("ADR018 Regions"),
	"pricing: Zero-config CDN":                 same("ADR029"),
	"pricing: Edge caching for web services":   same("ADR018 Edge caching"),
	"pricing: Custom domains":                  {kind: "meter", ref: "ADR030 §16, core.IncludedCustomDomains, pricing.yaml customDomains", check: checkCustomDomains},
	"pricing: Automatic TLS":                   same("ADR005"),
	"pricing: Wildcard domains":                same("ADR005 § Wildcard domains"),
	"pricing: WebSockets":                      same("ADR018"),
	"pricing: Service discovery":               same("ADR043"),
	"pricing: Dedicated IPs":                   nonGoal("DO_NOT_DO: dedicated outbound IPs; ADR018 Dedicated outbound IPs"),
	"pricing: AWS private links":               followUp(followUpHIPAALinks),
	"pricing: Private link outbound bandwidth": followUp(followUpHIPAALinks),
	// Protection
	"pricing: Automatic DDoS mitigation":     same("ADR018"),
	"pricing: Firewall":                      same("ADR018"),
	"pricing: HTTPS only":                    same("ADR005"),
	"pricing: Inbound IP rules":              gate(core.FeatureInboundIPRules),
	"pricing: Network-isolated environments": gate(core.FeatureNetworkIsolation),
	// Build & Deploy
	"pricing: Automatic builds": same("ADR018 Deploys"),
	"pricing: Standard build pipeline": numbered("meter", "ADR030 §13, pricing.yaml build.includedMinutes",
		leadingNumber, includedBuildMinutes),
	"pricing: Performance build pipeline":           gap("ADR018 Performance pipeline tier, w6/m310"),
	"pricing: Native runtimes":                      same("ADR018"),
	"pricing: Docker builds":                        same("ADR018"),
	"pricing: Environment variables & secret files": same("ADR018 Environment variables"),
	"pricing: Monorepo support":                     same("ADR018 Root Directory, Build Filters"),
	"pricing: Previews":                             nonGoal("DO_NOT_DO: PR preview environments"),
	"pricing: Single-service previews":              nonGoal("DO_NOT_DO: PR preview environments"),
	"pricing: Full-stack preview environments":      nonGoal("DO_NOT_DO: PR preview environments"),
	"pricing: Deploy orchestration":                 same("ADR004"),
	"pricing: Auto-deploys":                         same("ADR018 Auto-deploy"),
	"pricing: Zero-downtime deploys":                same("ADR004"),
	"pricing: Deploy hooks":                         same("ADR018 Deploy hooks"),
	"pricing: Pre-deploy command":                   same("ADR018 Pre-deploy command"),
	"pricing: Instant rollbacks":                    numbered("limit", "ADR060, core.KeptBuilds", leadingNumber, keptBuilds),
	"pricing: Projects":                             same("ADR018 Projects"),
	"pricing: Environments":                         numbered("limit", "core.MaxEnvironmentsPerProject", leadingNumber, func(p string) int64 { return unlimitedAtZero(core.MaxEnvironmentsPerProject(p)) }),
	"pricing: Protected environments":               same("ADR018 Protected environments"),
	"pricing: Infrastructure as code":               same("ADR049"),
	// Monitoring
	"pricing: SSH access":                   same("ADR035"),
	"pricing: Health checks":                same("ADR018 Health checks"),
	"pricing: Service metrics":              same("ADR018 Core metrics"),
	"pricing: Notifications":                same("ADR018 Notifications"),
	"pricing: Log retention":                numbered("retention", "core.logRetention (w9/m389)", leadingNumber, logRetentionDays),
	"pricing: Log streams":                  nonGoal("DO_NOT_DO: external log/metric drains; ADR018 Log streams"),
	"pricing: HTTP request logs":            gate(core.FeatureRequestLogs),
	"pricing: OpenTelemetry metrics stream": nonGoal("DO_NOT_DO: external log/metric drains; ADR018 Metric streams"),
	// Platform Access & Security
	"pricing: Team seats":                 numbered("limit", "store.LimitsFor MaxMembers", leadingNumber, func(p string) int64 { return unlimitedAtZero(store.LimitsFor(p).MaxMembers) }),
	"pricing: Workspaces":                 {kind: "limit", ref: "core.OrganizationPlan (ADR115, w7/m464); per user, store.LimitsFor MaxWorkspacesPerUser", check: checkWorkspaces},
	"pricing: 2FA & Google login":         same("ADR012"),
	"pricing: Enforce 2FA & Google login": gateBoth(core.FeatureTwoFactorEnforcement, core.FeatureGoogleLoginEnforcement),
	"pricing: Workspace user roles":       {kind: "limit", ref: "store.LimitsFor AllowedRoles", check: checkWorkspaceRoles},
	"pricing: Organization user roles":    {kind: "limit", ref: "core.OrganizationRoles, store.OrgRoles (ADR115 D4)", check: checkOrganizationRoles},
	"pricing: SAML SSO":                   nonGoal("DO_NOT_DO: SSO/SAML · SCIM; ADR018 SSO / SAML · SCIM"),
	"pricing: SCIM":                       nonGoal("DO_NOT_DO: SSO/SAML · SCIM; ADR018 SSO / SAML · SCIM"),
	"pricing: OIDC authentication":        gap("ADR018 Managed OIDC for AI providers, w2/m476–m480"),
	"pricing: Audit logs":                 gate(core.FeatureAuditLogs),
	// Compliance
	"pricing: GDPR DPA":                  gap(gapComplianceDocuments),
	"pricing: SOC 2 Type II":             gap(gapComplianceDocuments),
	"pricing: ISO 27001":                 gap(gapComplianceDocuments),
	"pricing: SOC 2 & ISO documentation": gap(gapComplianceDocuments),
	"pricing: HIPAA BAA":                 followUp(followUpHIPAALinks),
	// Support
	"pricing: Email":                                      commercial("support channel"),
	"pricing: Chat":                                       commercial("support channel"),
	"pricing: Premium support":                            commercial("support contract"),
	"pricing: Private Slack channel":                      commercial("support contract"),
	"pricing: Dedicated technical account team":           commercial("support contract"),
	"pricing: Support response SLAs":                      commercial("support contract"),
	"pricing: Migration assistance & architecture review": commercial("support contract"),

	// render.com/docs/platform-features-by-plan
	"features: Autoscaling":                   minimumGate(core.FeatureAutoscaling),
	"features: Preview environments":          nonGoal("DO_NOT_DO: PR preview environments"),
	"features: Performance build pipeline":    gap("ADR018 Performance pipeline tier, w6/m310"),
	"features: Dedicated outbound IPs":        nonGoal("DO_NOT_DO: dedicated outbound IPs"),
	"features: Network-isolated environments": minimumGate(core.FeatureNetworkIsolation),
	"features: Private links":                 followUp(followUpHIPAALinks),
	"features: Expanded inbound IP rules":     minimumGate(core.FeatureInboundIPRules),
	"features: HTTP request logs":             minimumGate(core.FeatureRequestLogs),
	"features: Response latency metrics":      minimumGate(core.FeatureLatencyMetrics),
	"features: Metrics streaming":             nonGoal("DO_NOT_DO: external log/metric drains"),
	"features: Log stream overrides":          nonGoal("DO_NOT_DO: external log/metric drains"),
	"features: Webhooks": {kind: "gate", ref: "core.planFeatures webhooks, core.WebhookDestinationLimit", check: func(t reporter, row planRow) {
		t.Helper()
		minimumGate(core.FeatureWebhooks).check(t, row)
		// "With a Pro plan, send webhook event notifications to one
		// destination. With a Scale or Enterprise plan, … up to 100."
		for plan, want := range map[string]int{store.PlanPro: 1, store.PlanScale: 100, store.PlanEnterprise: 100} {
			if got := core.WebhookDestinationLimit(plan); got != want {
				t.Errorf("%s: %s destinations = %d, want %d", row.key, plan, got, want)
			}
		}
	}},
	"features: Compliance documentation": gap(gapComplianceDocuments),
	"features: Audit logs":               minimumGate(core.FeatureAuditLogs),
	"features: OIDC":                     gap("ADR018 Managed OIDC for AI providers, w2/m476–m480"),
	"features: SAML SSO and SCIM":        nonGoal("DO_NOT_DO: SSO/SAML · SCIM"),
	"features: Org-level audit logs":     minimumGate(core.FeatureOrganizations),
	"features: Additional member roles": {kind: "limit", ref: "store.LimitsFor AllowedRoles", check: func(t reporter, row planRow) {
		t.Helper()
		for _, plan := range renderPlans {
			has := store.RoleAllowedOnPlan(plan, "contributor") && store.RoleAllowedOnPlan(plan, "viewer") && store.RoleAllowedOnPlan(plan, "billing")
			if want := slices.Index(renderPlans, plan) >= slices.Index(renderPlans, row.values[0]); has != want {
				t.Errorf("%s: %s has the Contributor/Viewer/Billing roles = %v, want %v", row.key, plan, has, want)
			}
		}
	}},
	"features: HIPAA-enabled workspaces": followUp(followUpHIPAALinks),
	"features: Unlimited workspace members": increased("store.LimitsFor MaxMembers", func(p string) int64 {
		return unlimitedTop(store.LimitsFor(p).MaxMembers)
	}),
	"features: Unlimited services": increased("store.LimitsFor MaxServices", func(p string) int64 {
		return unlimitedTop(store.LimitsFor(p).MaxServices)
	}),
	"features: Unlimited projects and environments": increased("core.MaxEnvironmentsPerProject", func(p string) int64 {
		return unlimitedTop(core.MaxEnvironmentsPerProject(p))
	}),
	"features: Increased number of included custom domains": increased("core.IncludedCustomDomains", func(p string) int64 {
		if n := includedDomains(p); n >= 0 {
			return n
		}
		return 1 << 62
	}),
	"features: Increased monthly included amounts of pipeline minutes and bandwidth usage": {kind: "meter", ref: "ADR030 §13, §14", check: func(t reporter, row planRow) {
		t.Helper()
		increased("", includedBuildMinutes).check(t, row)
		increased("", includedBandwidthGB).check(t, row)
	}},
	"features: Increased retention of past builds for rollbacks": increased("core.KeptBuilds", keptBuilds),
	"features: Increased retention of historical service metrics and logs": {kind: "retention", ref: "core.logRetention, core.metricsRetention", check: func(t reporter, row planRow) {
		t.Helper()
		increased("", logRetentionDays).check(t, row)
		increased("", metricsRetentionDays).check(t, row)
	}},
	"features: Increased per-minute task provisioning for Render Workflows": nonGoal("DO_NOT_DO: Workflows & tasks"),
}

// checkCustomDomains compares each plan's included count with
// core.IncludedCustomDomains, and the price beyond it ("then $0.25/domain/
// month") with pricing.yaml's at ADR030 §2's 30% discount.
func checkCustomDomains(t reporter, row planRow) {
	t.Helper()
	numbered("meter", "core.IncludedCustomDomains", leadingNumber, includedDomains).check(t, row)
	const monthSeconds = 730 * 3600
	for i, cell := range row.values {
		_, price, ok := strings.Cut(cell, "then $")
		if !ok {
			continue
		}
		render, err := strconv.ParseFloat(strings.TrimSuffix(price, "/domain/month"), 64)
		if err != nil {
			t.Errorf("%s on %s: unreadable price %q", row.key, renderPlans[i], cell)
			continue
		}
		if got, want := Default.customDomain*monthSeconds, render*0.70; math.Abs(got-want) > 1e-9 {
			t.Errorf("%s on %s: bex charges $%.4f a domain-month, want Render's $%.2f × 0.70 = $%.4f", row.key, renderPlans[i], got, render, want)
		}
	}
}

// organizationPlans are the plans whose workspaces sit in a Render
// organization: "With a Render Scale or Enterprise plan, you can manage all of
// your team's users" in one org, and "An org can contain unlimited workspaces
// at no additional cost" (render.com/docs/organizations, read 2026-11-17).
var organizationPlans = []string{store.PlanScale, store.PlanEnterprise}

// checkWorkspaces reads the pricing table's "Workspaces: 1 | 1 | Unlimited |
// Unlimited" row as what it counts on Render (w7/070): the workspaces one
// plan's account holds, a standalone workspace on Hobby and Pro and an
// organization's unlimited workspaces on Scale and Enterprise. bex binds it
// through organizations (w7/m464, ADR115): a Scale or Enterprise workspace
// is an org's primary, whose Owners create any number of workspaces on its
// one plan, and Hobby and Pro have no org (core.OrganizationPlan). How many
// workspaces one user may create is render.com/docs/team-members' rule (read
// 2026-11-17: "You can have up to five Hobby workspaces", "You can create an
// unlimited number of paid workspaces"), which store.LimitsFor applies.
func checkWorkspaces(t reporter, row planRow) {
	t.Helper()
	for i, plan := range renderPlans {
		want := "1"
		if slices.Contains(organizationPlans, plan) {
			want = "Unlimited"
		}
		if row.values[i] != want {
			t.Errorf("%s on %s: Render now shows %q, where bex read %q as an organization's workspace count: read the row again", row.key, plan, row.values[i], want)
		}
		if got := core.OrganizationPlan(plan); got != (want == "Unlimited") {
			t.Errorf("%s on %s: Render shows %q, but bex's OrganizationPlan = %v", row.key, plan, want, got)
		}
		perUser := 0 // unlimited
		if plan == store.PlanHobby {
			perUser = 5
		}
		if got := store.LimitsFor(plan).MaxWorkspacesPerUser; got != perUser {
			t.Errorf("%s on %s: bex lets a user own %d (0 = unlimited), Render's team-members page %d", row.key, plan, got, perUser)
		}
	}
}

// unlimitedTop orders a "0 = unlimited" cap so unlimited is the largest.
func unlimitedTop(n int) int64 {
	if n == 0 {
		return 1 << 62
	}
	return int64(n)
}

// checkWorkspaceRoles compares Render's role list per plan with the roles bex
// lets a plan assign. Hobby's blank cell is a single-member workspace whose
// one member is its admin.
func checkWorkspaceRoles(t reporter, row planRow) {
	t.Helper()
	for i, cell := range row.values {
		want := []string{"admin"}
		if cell != "-" {
			want = nil
			for _, role := range strings.Split(cell, ",") {
				want = append(want, strings.ToLower(strings.TrimSpace(role)))
			}
		}
		if got := store.LimitsFor(renderPlans[i]).AllowedRoles; !slices.Equal(got, want) {
			t.Errorf("%s on %s: Render %q, bex %v", row.key, renderPlans[i], cell, got)
		}
	}
}

// checkOrganizationRoles holds "Organization user roles: — | — | Owner,
// Member, Guest | Owner, Member, Guest" to the org roles bex assigns
// (ADR115 D4): none where there is no org, and the store's whole role set
// where there is one.
func checkOrganizationRoles(t reporter, row planRow) {
	t.Helper()
	for i, cell := range row.values {
		var want []string
		if cell != "-" {
			for _, role := range strings.Split(cell, ",") {
				want = append(want, strings.ToLower(strings.TrimSpace(role)))
			}
		}
		got := core.OrganizationRoles(renderPlans[i])
		if !slices.Equal(got, want) {
			t.Errorf("%s on %s: Render %q, bex %v", row.key, renderPlans[i], cell, got)
		}
		if want != nil && !slices.Equal(store.OrgRoles, want) {
			t.Errorf("%s on %s: Render %q, the store's org roles %v", row.key, renderPlans[i], cell, store.OrgRoles)
		}
	}
}

// readTSV reads a captured table: tab-separated, # comments, width cells a
// row.
func readTSV(t *testing.T, path string, width int) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var rows [][]string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cells := strings.Split(line, "\t")
		if len(cells) != width {
			t.Fatalf("%s: %q has %d cells, want %d", path, line, len(cells), width)
		}
		rows = append(rows, cells)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// readPlanTable reads a captured plan table: the first two columns the
// section and the row title, the rest its values.
func readPlanTable(t *testing.T, path, table string, width int) []planRow {
	t.Helper()
	var rows []planRow
	for _, cells := range readTSV(t, path, 2+width) {
		rows = append(rows, planRow{key: table + ": " + cells[1], values: cells[2:]})
	}
	return rows
}

// undispositioned is every captured row with no disposition, and every
// disposition whose row is no longer captured.
func undispositioned(rows []planRow, dispositions map[string]planDisposition) (missing, stale []string) {
	captured := map[string]bool{}
	for _, row := range rows {
		if captured[row.key] {
			missing = append(missing, row.key+" (captured twice)")
		}
		captured[row.key] = true
		if _, ok := dispositions[row.key]; !ok {
			missing = append(missing, row.key)
		}
	}
	for key := range dispositions {
		if !captured[key] {
			stale = append(stale, key)
		}
	}
	slices.Sort(stale)
	return missing, stale
}

func capturedPlanRows(t *testing.T, dir string) []planRow {
	t.Helper()
	return append(readPlanTable(t, filepath.Join(dir, "render-plan-table.tsv"), "pricing", 4),
		readPlanTable(t, filepath.Join(dir, "render-features-by-plan.tsv"), "features", 1)...)
}

func TestEveryRenderPlanRowHasADisposition(t *testing.T) {
	rows := capturedPlanRows(t, "testdata")
	if len(rows) < 100 {
		t.Fatalf("captured %d rows; the two tables hold 119", len(rows))
	}
	missing, stale := undispositioned(rows, planDispositions)
	for _, key := range missing {
		t.Errorf("Render plan row %q has no disposition in bex: add a gate, limit, meter, retention, non-goal or filed follow-up", key)
	}
	for _, key := range stale {
		t.Errorf("disposition %q names a row Render's captured tables no longer have", key)
	}
	kinds := []string{"same", "gate", "limit", "meter", "retention", "nonGoal", "gap", "followUp", "commercial"}
	for _, row := range rows {
		d, ok := planDispositions[row.key]
		if !ok {
			continue
		}
		if !slices.Contains(kinds, d.kind) || d.ref == "" {
			t.Errorf("%s: disposition %+v needs one of %v and a reference", row.key, d, kinds)
		}
		if d.check != nil {
			t.Run(row.key, func(t *testing.T) { d.check(t, row) })
		}
	}
}

// The guard itself: a row added to the captured table with no disposition is
// reported, and so is a disposition for a row that is gone.
func TestPlanTableGuardReportsUndispositionedRows(t *testing.T) {
	rows := capturedPlanRows(t, "testdata")
	added := append(slices.Clone(rows), planRow{key: "pricing: Teleportation", values: []string{"-", "yes", "yes", "yes"}})
	if missing, _ := undispositioned(added, planDispositions); !slices.Equal(missing, []string{"pricing: Teleportation"}) {
		t.Fatalf("an undispositioned row must be reported, got %v", missing)
	}
	removed := slices.DeleteFunc(slices.Clone(rows), func(r planRow) bool { return r.key == "pricing: Instant rollbacks" })
	if _, stale := undispositioned(removed, planDispositions); !slices.Equal(stale, []string{"pricing: Instant rollbacks"}) {
		t.Fatalf("a disposition whose row is gone must be reported, got %v", stale)
	}
}

// A numbered disposition fails when the code's number moves off Render's.
func TestPlanTableGuardComparesNumbers(t *testing.T) {
	row := planRow{key: "pricing: Instant rollbacks", values: []string{"5 builds retained", "15 builds retained", "30 builds retained", "30 builds retained"}}
	var kept failures
	numbered("limit", "test", leadingNumber, keptBuilds).check(&kept, row)
	var moved failures
	numbered("limit", "test", leadingNumber, func(p string) int64 { return keptBuilds(p) + 1 }).check(&moved, row)
	if kept.failed || !moved.failed {
		t.Fatalf("the guard must pass on bex's counts (failed=%v) and fail on a count off Render's (failed=%v)", kept.failed, moved.failed)
	}
	for cell, want := range map[string]int64{"2 included then $0.25/domain/month": 2, "1K mins per month then $5 per 1K mins": 1000, "1 TB included per month then $0.15 per GB": 1000, "Unlimited": -1, "2 per project": 2, "1,000 Up to 5,000 on request": 1000} {
		if got, ok := leadingNumber(cell); !ok || got != want {
			t.Errorf("leadingNumber(%q) = %d, %v, want %d", cell, got, ok, want)
		}
	}
	if _, ok := leadingNumber("Custom"); ok {
		t.Error(`"Custom" binds no number`)
	}
}
