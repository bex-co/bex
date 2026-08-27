import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  buildLoginRedirectHref,
  handleUnauthenticated,
  resetAuthRedirectForTests,
} from "../auth-redirect";
import { fetchSession, invalidateSessionCache } from "@/common/server-fn/session";

vi.mock("@/common/server-fn/session", () => ({
  fetchSession: vi.fn(),
  invalidateSessionCache: vi.fn(),
}));

const mockFetchSession = vi.mocked(fetchSession);
const mockInvalidate = vi.mocked(invalidateSessionCache);

describe("buildLoginRedirectHref (w3/m80 t001)", () => {
  it("carries the current location back as next", () => {
    expect(buildLoginRedirectHref("/services/srv-1?tab=env", false)).toBe(
      "/auth/login?next=%2Fservices%2Fsrv-1%3Ftab%3Denv",
    );
  });

  it("adds aal=aal2 when a second factor is owed", () => {
    const href = buildLoginRedirectHref("/x", true);
    expect(href).toContain("next=%2Fx");
    expect(href).toContain("aal=aal2");
  });

  it("omits next when there is no current href", () => {
    expect(buildLoginRedirectHref("", false)).toBe("/auth/login");
  });
});

describe("handleUnauthenticated (w3/m80 t001)", () => {
  let assign: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.clearAllMocks();
    resetAuthRedirectForTests();
    window.history.replaceState(null, "", "/services/srv-1?tab=env");
    assign = vi.fn();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        pathname: "/services/srv-1",
        search: "?tab=env",
        hash: "",
        assign,
      },
    });
  });

  afterEach(() => {
    resetAuthRedirectForTests();
  });

  it("redirects to login with next when the session is gone", async () => {
    mockFetchSession.mockResolvedValue({ session: null, aal2Required: false });

    await handleUnauthenticated();

    expect(mockInvalidate).toHaveBeenCalled();
    expect(assign).toHaveBeenCalledWith(
      "/auth/login?next=%2Fservices%2Fsrv-1%3Ftab%3Denv",
    );
  });

  it("passes aal=aal2 through when the re-check owes a second factor", async () => {
    mockFetchSession.mockResolvedValue({ session: null, aal2Required: true });

    await handleUnauthenticated();

    expect(assign).toHaveBeenCalledWith(expect.stringContaining("aal=aal2"));
  });

  it("does NOT redirect when the session is still live (a 401 that wasn't an expiry)", async () => {
    mockFetchSession.mockResolvedValue({
      session: { id: "s" } as never,
      aal2Required: false,
    });

    await handleUnauthenticated();

    expect(assign).not.toHaveBeenCalled();
  });

  it("redirects at most once across a burst of concurrent 401s", async () => {
    mockFetchSession.mockResolvedValue({ session: null, aal2Required: false });

    await Promise.all([
      handleUnauthenticated(),
      handleUnauthenticated(),
      handleUnauthenticated(),
    ]);

    expect(assign).toHaveBeenCalledTimes(1);
  });

  it("never bounces login → login", async () => {
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        pathname: "/auth/login",
        search: "",
        hash: "",
        assign,
      },
    });
    mockFetchSession.mockResolvedValue({ session: null, aal2Required: false });

    await handleUnauthenticated();

    expect(mockFetchSession).not.toHaveBeenCalled();
    expect(assign).not.toHaveBeenCalled();
  });
});
