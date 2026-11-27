import { describe, expect, it } from "vitest";
import { Route as AgentsRoute } from "../agents";
import { Route as AgentDetailRoute } from "../agents_.$agentSessionId";
import { Route as BuildSourcesRoute } from "../build-sources";
import { Route as BuildSourcesNewRoute } from "../build-sources_.new";
import { Route as DatabaseIndexRoute } from "../databases.$databaseId.index";
import { Route as DatabaseLogsRoute } from "../databases.$databaseId.logs";
import { Route as DatabaseMetricsRoute } from "../databases.$databaseId.metrics";
import { Route as IndexRoute } from "../index";
import { Route as KeyValueIndexRoute } from "../keyvalue.$keyValueId.index";
import { Route as KeyValueLogsRoute } from "../keyvalue.$keyValueId.logs";
import { Route as KeyValueMetricsRoute } from "../keyvalue.$keyValueId.metrics";
import { Route as ServiceDeployRoute } from "../services.$serviceId.deploys.$deployId";
import { Route as ServiceLogsRoute } from "../services.$serviceId.logs";
import { Route as ServiceMetricsRoute } from "../services.$serviceId.metrics";
import { Route as ServiceShellRoute } from "../services.$serviceId.shell";
import { Route as ProjectApplicationLogsRoute } from "../project.$projectId.observability.logs.application";
import { Route as ProjectRequestLogsRoute } from "../project.$projectId.observability.logs.request";
import { Route as ProjectMetricsRoute } from "../project.$projectId.observability.metrics";
import { Route as ProjectOverviewRoute } from "../project.$projectId._overview";
import { Route as ProjectSettingsRoute } from "../project.$projectId.settings";
import { Route as SandboxGroupIndexRoute } from "../sandbox-groups_.$sandboxGroupId.index";
import { Route as SandboxGroupImagesRoute } from "../sandbox-groups_.$sandboxGroupId.images";
import { Route as SandboxGroupSnapshotsRoute } from "../sandbox-groups_.$sandboxGroupId.snapshots";
import { Route as SettingsRoute } from "../settings";
import { Route as StaticDeployRoute } from "../static.$serviceId.deploys.$deployId";
import { Route as WorkspaceSettingsRoute } from "../workspace.settings";

// The router builds a match's search as {...raw, ...validated}, so a validator
// that omits a rejected key hands its raw value to every reader (w5/078,
// w5/100): `?r=12h` threw in the deploy log viewer, `?type=bogus` reached
// bex-api from the Logs tab, `?git_error=1` showed a GitHub error on /settings.
// Each validator returns exactly the keys it owns, undefined when rejected.
// toStrictEqual is the point: toEqual cannot tell an omitted key from an
// undefined one. The expectations are spelled out rather than built from the
// code's own constants, so a key dropped there fails here.
const NO_RANGE = {
  range: undefined,
  rangeStart: undefined,
  rangeEnd: undefined,
};
// The datastore Logs tab's own keys (w4/228, w5/m139): range + text +
// instance + level, and View in context's marked line (w3/m528).
const NO_DATASTORE_LOG_FILTERS = {
  ...NO_RANGE,
  text: undefined,
  instance: undefined,
  level: undefined,
  contextLine: undefined,
};
const NO_LOG_FILTERS = {
  t: undefined,
  r: undefined,
  ...NO_RANGE,
  type: undefined,
  level: undefined,
  method: undefined,
  statusCode: undefined,
  instance: undefined,
  path: undefined,
  host: undefined,
  text: undefined,
  live: undefined,
  contextLine: undefined,
};
// A project log view's keys (w3/m505), and View in context's marked line.
const NO_PROJECT_LOG_SEARCH = {
  savedSearchId: undefined,
  r: undefined,
  rangeStart: undefined,
  rangeEnd: undefined,
  contextLine: undefined,
};

describe.each([
  [
    "/agents",
    AgentsRoute,
    { phase: "bogus", view: "grid", archived: "no", prState: "draft" },
    {
      view: undefined,
      archived: undefined,
      phase: undefined,
      tag: undefined,
      prState: undefined,
    },
  ],
  // The list's PR-state filter (w9/m386).
  [
    "/agents",
    AgentsRoute,
    { prState: "merged" },
    {
      view: undefined,
      archived: undefined,
      phase: undefined,
      tag: undefined,
      prState: "merged",
    },
  ],
  [
    "/agents/$agentSessionId",
    AgentDetailRoute,
    { fromPhase: "bogus", fromArchived: 1 },
    { fromArchived: undefined, fromPhase: undefined },
  ],
  [
    "/settings",
    SettingsRoute,
    { git_error: 1, addKey: 1, returnTo: 123, flow: 7, git_claim_selection: 2 },
    {
      flow: undefined,
      git_error: undefined,
      git_claim_selection: undefined,
      returnTo: undefined,
      addKey: undefined,
    },
  ],
  // A Key Value instance's bare URL owns only the old ?tab=logs and
  // ?tab=metrics, which it sends to their pages (w3/m649); the Logs and
  // Metrics pages own their keys.
  [
    "/keyvalue/$keyValueId/",
    KeyValueIndexRoute,
    { tab: "bogus", range: "12y", text: "", instance: true },
    { tab: undefined },
  ],
  [
    "/keyvalue/$keyValueId/",
    KeyValueIndexRoute,
    { tab: "metrics", range: "24h" },
    { tab: "metrics" },
  ],
  [
    "/keyvalue/$keyValueId/logs",
    KeyValueLogsRoute,
    { tab: "logs", range: "12y", text: "", instance: true },
    NO_DATASTORE_LOG_FILTERS,
  ],
  [
    "/keyvalue/$keyValueId/metrics",
    KeyValueMetricsRoute,
    { range: "4h", rangeStart: "x", text: "boom" },
    { ...NO_RANGE, range: "4h" },
  ],
  // A Postgres database's bare URL owns only the old ?tab=logs, which it
  // sends to the Logs page (w3/m641); the Logs and Metrics pages own their
  // keys.
  [
    "/databases/$databaseId/",
    DatabaseIndexRoute,
    { tab: "metrics", range: "24h", text: 404 },
    { tab: undefined },
  ],
  [
    "/databases/$databaseId/logs",
    DatabaseLogsRoute,
    { tab: "logs", range: "custom", rangeStart: "x", text: 404 },
    { ...NO_DATASTORE_LOG_FILTERS, text: "404" },
  ],
  [
    "/databases/$databaseId/metrics",
    DatabaseMetricsRoute,
    { range: "4h", rangeStart: "x", text: "boom" },
    { ...NO_RANGE, range: "4h" },
  ],
  ["/", IndexRoute, { new: "bogus" }, { new: undefined }],
  // The three create dialogs the overview owns in its URL (w1/m45, w22/m311).
  ["/", IndexRoute, { new: "database" }, { new: "database" }],
  ["/", IndexRoute, { new: "project" }, { new: "project" }],
  ["/", IndexRoute, { new: "sandbox-group" }, { new: "sandbox-group" }],
  [
    "/workspace/settings",
    WorkspaceSettingsRoute,
    { plan: "upgrade" },
    { plan: undefined },
  ],
  // The project page's pre-w1/m682 keys, read only to redirect them.
  [
    "/project/$projectId",
    ProjectOverviewRoute,
    { env: "env-davm6imde41s73canq0g", q: "", kind: 3, other: "x" },
    { env: "evm-davm6imde41s73canq0g", q: undefined, kind: undefined },
  ],
  [
    "/project/$projectId/settings",
    ProjectSettingsRoute,
    { edit: "name" },
    { edit: "name" },
  ],
  [
    "/project/$projectId/settings",
    ProjectSettingsRoute,
    { edit: "delete" },
    { edit: undefined },
  ],
  [
    "/services/$serviceId/metrics",
    ServiceMetricsRoute,
    { range: "1h", rangeStart: "2026-10-01T00:00:00Z" },
    { ...NO_RANGE, range: "1h" },
  ],
  [
    "/services/$serviceId/logs",
    ServiceLogsRoute,
    { type: "bogus", r: "1h", level: true, live: "yes" },
    { ...NO_LOG_FILTERS, range: "1h" },
  ],
  // View in context's link (w3/m528): a custom window, its line marked by
  // key. A link written before w3/m537 carries the Live switch's `live=0`,
  // which is a tombstone now: a custom range never tails.
  [
    "/services/$serviceId/logs",
    ServiceLogsRoute,
    {
      range: "custom",
      rangeStart: "2026-11-20T10:00:00.000Z",
      rangeEnd: "2026-11-20T10:20:00.000Z",
      live: 0,
      contextLine: "id:abc",
    },
    {
      ...NO_LOG_FILTERS,
      range: "custom",
      rangeStart: "2026-11-20T10:00:00.000Z",
      rangeEnd: "2026-11-20T10:20:00.000Z",
      contextLine: "id:abc",
    },
  ],
  // Live tail is a range (w3/m537): Render's `live`, in bex's `range` key
  // or Render's `r`.
  [
    "/services/$serviceId/logs",
    ServiceLogsRoute,
    { r: "live", live: 0 },
    { ...NO_LOG_FILTERS, range: "live" },
  ],
  [
    "/databases/$databaseId/logs",
    DatabaseLogsRoute,
    { range: "live", rangeEnd: "2026-11-20T10:20:00.000Z" },
    { ...NO_DATASTORE_LOG_FILTERS, range: "live" },
  ],
  [
    "/project/$projectId/observability/logs/application",
    ProjectApplicationLogsRoute,
    { savedSearchId: "svs-1", r: "live" },
    { ...NO_PROJECT_LOG_SEARCH, savedSearchId: "svs-1", r: "live" },
  ],
  // A Service Metrics view has no Live tail.
  [
    "/project/$projectId/observability/metrics",
    ProjectMetricsRoute,
    { savedSearchId: "svs-1", r: "live" },
    { ...NO_PROJECT_LOG_SEARCH, savedSearchId: "svs-1" },
  ],
  [
    "/databases/$databaseId/logs",
    DatabaseLogsRoute,
    { contextLine: "id:abc" },
    { ...NO_DATASTORE_LOG_FILTERS, contextLine: "id:abc" },
  ],
  [
    "/keyvalue/$keyValueId/logs",
    KeyValueLogsRoute,
    { contextLine: true },
    NO_DATASTORE_LOG_FILTERS,
  ],
  [
    "/project/$projectId/observability/logs/application",
    ProjectApplicationLogsRoute,
    { savedSearchId: "svs-1", contextLine: "id:abc", text: "x" },
    { ...NO_PROJECT_LOG_SEARCH, savedSearchId: "svs-1", contextLine: "id:abc" },
  ],
  [
    "/project/$projectId/observability/logs/request",
    ProjectRequestLogsRoute,
    { contextLine: 1 },
    { ...NO_PROJECT_LOG_SEARCH, contextLine: "1" },
  ],
  // SSH into instance opens the Shell on a line's instance (w3/m528).
  [
    "/services/$serviceId/shell",
    ServiceShellRoute,
    { instance: "srv-abc-0123456789abcdefghij", tab: "x" },
    { instance: "srv-abc-0123456789abcdefghij" },
  ],
  [
    "/services/$serviceId/shell",
    ServiceShellRoute,
    { instance: true },
    { instance: undefined },
  ],
  [
    "/services/$serviceId/deploys/$deployId",
    ServiceDeployRoute,
    { r: "12h" },
    { r: undefined },
  ],
  [
    "/services/$serviceId/deploys/$deployId",
    ServiceDeployRoute,
    { r: "1h" },
    { r: "1h" },
  ],
  [
    "/static/$serviceId/deploys/$deployId",
    StaticDeployRoute,
    { r: "12h" },
    { r: undefined },
  ],
  [
    "/build-sources",
    BuildSourcesRoute,
    { q: "  ", kind: "bogus", extra: "ignored" },
    { q: undefined, kind: undefined },
  ],
  [
    "/build-sources",
    BuildSourcesRoute,
    { q: "api", kind: "git" },
    { q: "api", kind: "git" },
  ],
  [
    "/build-sources/new",
    BuildSourcesNewRoute,
    { projectId: "   " },
    { projectId: undefined },
  ],
  [
    "/build-sources/new",
    BuildSourcesNewRoute,
    { projectId: "prj-123" },
    { projectId: "prj-123" },
  ],
  // A sandbox group's drawers (w1/m658): a sandbox, snapshot or image and
  // the drawer's tab; a malformed id or an unknown tab is dropped.
  [
    "/sandbox-groups/$sandboxGroupId/",
    SandboxGroupIndexRoute,
    { sandbox: "sbx-dc44tha9086ovmjtv6c0", tab: "snapshots", extra: 1 },
    { sandbox: "sbx-dc44tha9086ovmjtv6c0", tab: "snapshots" },
  ],
  [
    "/sandbox-groups/$sandboxGroupId/",
    SandboxGroupIndexRoute,
    { sandbox: "srv-123", tab: "shell" },
    { sandbox: undefined, tab: undefined },
  ],
  [
    "/sandbox-groups/$sandboxGroupId/snapshots",
    SandboxGroupSnapshotsRoute,
    { snapshot: "snp-dc44tha9086ovmjtv6c0" },
    { snapshot: "snp-dc44tha9086ovmjtv6c0" },
  ],
  [
    "/sandbox-groups/$sandboxGroupId/snapshots",
    SandboxGroupSnapshotsRoute,
    { snapshot: 7 },
    { snapshot: undefined },
  ],
  [
    "/sandbox-groups/$sandboxGroupId/images",
    SandboxGroupImagesRoute,
    { image: "agent-runtime", tab: "sandboxes" },
    { image: "agent-runtime", tab: "sandboxes" },
  ],
  [
    "/sandbox-groups/$sandboxGroupId/images",
    SandboxGroupImagesRoute,
    { image: "  ", tab: "logs" },
    { image: undefined, tab: undefined },
  ],
])("%s validates %j", (_path, route, input, want) => {
  it("to exactly the keys it owns", () => {
    const validate = route.options.validateSearch as (
      search: Record<string, unknown>,
    ) => Record<string, unknown>;
    expect(validate(input)).toStrictEqual(want);
  });
});
