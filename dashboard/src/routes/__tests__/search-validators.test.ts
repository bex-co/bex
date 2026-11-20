import { describe, expect, it } from "vitest";
import { Route as AgentsRoute } from "../agents";
import { Route as AgentDetailRoute } from "../agents_.$agentSessionId";
import { Route as DatabaseRoute } from "../databases.$databaseId";
import { Route as IndexRoute } from "../index";
import { Route as KeyValueRoute } from "../keyvalue.$keyValueId";
import { Route as ServiceDeployRoute } from "../services.$serviceId.deploys.$deployId";
import { Route as ServiceLogsRoute } from "../services.$serviceId.logs";
import { Route as ServiceMetricsRoute } from "../services.$serviceId.metrics";
import { Route as ServiceShellRoute } from "../services.$serviceId.shell";
import { Route as ProjectApplicationLogsRoute } from "../project.$projectId.observability.logs.application";
import { Route as ProjectRequestLogsRoute } from "../project.$projectId.observability.logs.request";
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
  [
    "/keyvalue/$keyValueId",
    KeyValueRoute,
    { tab: "bogus", range: "12y", text: "", instance: true },
    { tab: undefined, ...NO_DATASTORE_LOG_FILTERS },
  ],
  [
    "/databases/$databaseId",
    DatabaseRoute,
    { tab: "metrics", range: "custom", rangeStart: "x", text: 404 },
    { tab: undefined, ...NO_DATASTORE_LOG_FILTERS, text: "404" },
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
  // View in context's link (w3/m528): a custom window, paused, its line
  // marked by key.
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
      live: 0,
      contextLine: "id:abc",
    },
  ],
  [
    "/databases/$databaseId",
    DatabaseRoute,
    { tab: "logs", contextLine: "id:abc" },
    { tab: "logs", ...NO_DATASTORE_LOG_FILTERS, contextLine: "id:abc" },
  ],
  [
    "/keyvalue/$keyValueId",
    KeyValueRoute,
    { tab: "logs", contextLine: true },
    { tab: "logs", ...NO_DATASTORE_LOG_FILTERS },
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
])("%s validates %j", (_path, route, input, want) => {
  it("to exactly the keys it owns", () => {
    const validate = route.options.validateSearch as (
      search: Record<string, unknown>,
    ) => Record<string, unknown>;
    expect(validate(input)).toStrictEqual(want);
  });
});
