import { describe, expect, it } from "vitest";
import { Route as AgentsRoute } from "../agents";
import { Route as AgentDetailRoute } from "../agents_.$agentSessionId";
import type { AgentsSearch } from "../agents";
import type {
  AgentSessionArchivedFilter,
  AgentSessionPhase,
} from "@/features/agent-sessions/types";

describe("agent-session filter search contracts", () => {
  const validateList = AgentsRoute.options.validateSearch as (
    search: Record<string, unknown>,
  ) => AgentsSearch;
  const validateDetail = AgentDetailRoute.options.validateSearch as (
    search: Record<string, unknown>,
  ) => {
    fromArchived?: AgentSessionArchivedFilter;
    fromPhase?: AgentSessionPhase;
  };

  it("keeps shareable membership and phase filters on the list", () => {
    expect(
      validateList({ archived: "true", phase: "failed", view: "list" }),
    ).toEqual({ archived: "true", phase: "failed", view: "list" });
    expect(validateList({ archived: "no", phase: "unknown" })).toEqual({});
  });

  it("keeps only valid list context on a detail URL", () => {
    expect(
      validateDetail({ fromArchived: "all", fromPhase: "completed" }),
    ).toEqual({ fromArchived: "all", fromPhase: "completed" });
    expect(
      validateDetail({ fromArchived: "false", fromPhase: "unknown" }),
    ).toEqual({});
  });
});
