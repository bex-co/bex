import { describe, expect, it } from "vitest";
import { Route as AgentsRoute } from "../agents";
import { Route as AgentDetailRoute } from "../agents_.$agentSessionId";
import { Route as DatabaseRoute } from "../databases.$databaseId";
import { Route as IndexRoute } from "../index";
import { Route as KeyValueRoute } from "../keyvalue.$keyValueId";
import { Route as ServiceDeployRoute } from "../services.$serviceId.deploys.$deployId";
import { Route as ServiceLogsRoute } from "../services.$serviceId.logs";
import { Route as ServiceMetricsRoute } from "../services.$serviceId.metrics";
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
  text: undefined,
  live: undefined,
};

describe.each([
  [
    "/agents",
    AgentsRoute,
    { phase: "bogus", view: "grid", archived: "no" },
    { view: undefined, archived: undefined, phase: undefined },
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
    { tab: "bogus", range: "12y" },
    { tab: undefined, ...NO_RANGE },
  ],
  [
    "/databases/$databaseId",
    DatabaseRoute,
    { tab: "metrics", range: "custom", rangeStart: "x" },
    { tab: undefined, ...NO_RANGE },
  ],
  ["/", IndexRoute, { new: "bogus" }, { new: undefined }],
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
