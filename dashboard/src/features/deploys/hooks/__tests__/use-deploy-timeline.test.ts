import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useDeployTimeline } from "../use-deploy-timeline";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

beforeEach(() => mockUseQuery.mockReset());

describe("useDeployTimeline", () => {
  it("windows the service-events query and keeps only the requested deploy id", () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          {
            id: "evt-right",
            type: "deploy_started",
            timestamp: "2026-07-14T00:00:00Z",
            details: { deployId: "dep-right", deployStatus: "" },
          },
          {
            id: "evt-other",
            type: "deploy_ended",
            timestamp: "2026-07-14T00:00:02Z",
            details: { deployId: "dep-other", deployStatus: "succeeded" },
          },
          {
            id: "evt-audit",
            type: "env_vars_changed",
            timestamp: "2026-07-14T00:00:01Z",
            details: { deployId: null, deployStatus: null },
          },
        ],
      },
      loading: false,
      error: undefined,
    });

    const { result } = renderHook(() =>
      useDeployTimeline(
        "web",
        "dep-right",
        "2026-07-14T00:00:00Z",
        "2026-07-14T00:05:00Z",
      ),
    );

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: {
          serviceId: "web",
          startTime: "2026-07-14T00:00:00Z",
          endTime: "2026-07-14T00:05:00Z",
          limit: 100,
        },
        pollInterval: 0,
      }),
    );
    expect(result.current.events.map((event) => event.id)).toEqual([
      "evt-right",
    ]);
  });

  it("polls while the deploy window is still open", () => {
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [] },
      loading: false,
      error: undefined,
    });

    renderHook(() =>
      useDeployTimeline("web", "dep-1", "2026-07-14T00:00:00Z", undefined),
    );

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ pollInterval: 3000 }),
    );
  });
});
