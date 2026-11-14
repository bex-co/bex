import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useQuery } from "@apollo/client/react";
import { useWebhook } from "../use-webhook";

vi.mock("@apollo/client/react", () => ({
  useQuery: vi.fn(),
}));

beforeEach(() => {
  vi.mocked(useQuery).mockReset();
});

describe("useWebhook", () => {
  // w4/443: a webhook id resolves its own workspace server-side; pinning the
  // selected workspace as ownerId sent a link to another one home.
  it("reads by id alone, never pinning the selected workspace", () => {
    vi.mocked(useQuery).mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    } as never);

    const { result } = renderHook(() => useWebhook("whk-elsewhere"));

    const options = vi.mocked(useQuery).mock.calls[0][1] as {
      variables: Record<string, unknown>;
      skip?: boolean;
    };
    expect(options.variables).toEqual({ id: "whk-elsewhere" });
    expect(options.skip).toBeFalsy();
    expect(result.current.loading).toBe(true);
  });

  // w4/641: the detail page selects the endpoint's own workspace.
  it("carries the endpoint's owning workspace onto the view", () => {
    vi.mocked(useQuery).mockReturnValue({
      data: {
        webhookEndpoint: { id: "whk-1", name: "hook", ownerId: "tea-2" },
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    } as never);

    const { result } = renderHook(() => useWebhook("whk-1"));

    expect(result.current.endpoint?.ownerId).toBe("tea-2");
  });
});
