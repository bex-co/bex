import { describe, expect, it } from "vitest";
import { Route as ServiceDeployRoute } from "../services.$serviceId.deploys.$deployId";
import { Route as StaticDeployRoute } from "../static.$serviceId.deploys.$deployId";

// w5/078: the router merges a route's validated search over the raw query, so
// an omitted key keeps its raw value. An unknown `?r=12h` reached the deploy
// log viewer that way and threw computing the range's start.
describe.each([
  ["services", ServiceDeployRoute],
  ["static", StaticDeployRoute],
])("%s deploy page ?r=", (_name, route) => {
  const validate = route.options.validateSearch as (
    search: Record<string, unknown>,
  ) => { r?: string };

  it("keeps a known log range", () => {
    expect(validate({ r: "1h" })).toStrictEqual({ r: "1h" });
  });

  it("sets an unknown range to undefined rather than omitting it", () => {
    expect(validate({ r: "12h" })).toStrictEqual({ r: undefined });
    expect(validate({})).toStrictEqual({ r: undefined });
  });
});
