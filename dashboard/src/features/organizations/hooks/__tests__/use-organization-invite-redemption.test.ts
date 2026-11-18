import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { INVITE_TOKEN_STORAGE_KEY } from "@/common/lib/invite-token";

const mocks = vi.hoisted(() => ({
  mutate: vi.fn(),
  query: vi.fn(),
  previewRefetch: vi.fn(),
  preview: {
    data: {
      organizationInvitePreview: {
        organizationId: "org-target",
        organizationName: "Acme Org",
        role: "MEMBER",
        alreadyMember: false,
      },
    },
    loading: false,
    error: undefined as unknown,
  },
}));
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mocks.mutate],
  useQuery: () => ({ ...mocks.preview, refetch: mocks.previewRefetch }),
  useApolloClient: () => ({ query: mocks.query }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn() } }));
import { useOrganizationInviteRedemption } from "../use-organization-invite-redemption";

// A well-formed capability built by expression, so no literal looks like a key.
const TOKEN = "ab".repeat(16);

function graphQLError(code: string) {
  return new CombinedGraphQLErrors({
    errors: [{ message: code, extensions: { code } }],
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.preview.data.organizationInvitePreview.alreadyMember = false;
  mocks.preview.error = undefined;
  mocks.mutate.mockResolvedValue({
    data: { acceptOrganizationInvite: { organizationId: "org-target" } },
  });
  mocks.query.mockResolvedValue({
    data: { organizations: [{ id: "org-target" }] },
  });
  window.history.replaceState(null, "", "/invite");
  window.sessionStorage.clear();
  window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, `org:${TOKEN}`);
});

// w7/083: an organization invite's link, accepted for whoever is signed in.
describe("organization invitation redemption", () => {
  it("never accepts on mount", () => {
    renderHook(() => useOrganizationInviteRedemption(TOKEN, vi.fn()));
    expect(mocks.mutate).not.toHaveBeenCalled();
  });

  it("opens the organization once the caller's organizations list it", async () => {
    const opened = vi.fn();
    const { result } = renderHook(() =>
      useOrganizationInviteRedemption(TOKEN, opened),
    );
    await act(() => result.current.accept());
    expect(mocks.mutate).toHaveBeenCalledWith({ variables: { token: TOKEN } });
    expect(opened).toHaveBeenCalledWith("org-target");
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
  });

  it("opens an org the caller is already in without accepting again", async () => {
    mocks.preview.data.organizationInvitePreview.alreadyMember = true;
    const opened = vi.fn();
    const { result } = renderHook(() =>
      useOrganizationInviteRedemption(TOKEN, opened),
    );
    expect(result.current.joined).toBe(true);
    await act(() => result.current.accept());
    expect(mocks.mutate).not.toHaveBeenCalled();
    expect(opened).toHaveBeenCalledWith("org-target");
  });

  it("retries opening, never the acceptance, while access is not ready", async () => {
    mocks.query.mockResolvedValueOnce({ data: { organizations: [] } });
    const opened = vi.fn();
    const { result } = renderHook(() =>
      useOrganizationInviteRedemption(TOKEN, opened),
    );
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBe("invites.orgAccessPending");
    expect(opened).not.toHaveBeenCalled();
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(
      `org:${TOKEN}`,
    );
    await act(() => result.current.accept());
    expect(mocks.mutate).toHaveBeenCalledOnce();
    expect(opened).toHaveBeenCalledWith("org-target");
  });

  it("refuses a link someone else already used, by name", async () => {
    mocks.mutate.mockRejectedValueOnce(graphQLError("INVITE_ALREADY_ACCEPTED"));
    mocks.previewRefetch.mockRejectedValueOnce(
      graphQLError("INVITE_ALREADY_ACCEPTED"),
    );
    const opened = vi.fn();
    const { result } = renderHook(() =>
      useOrganizationInviteRedemption(TOKEN, opened),
    );
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBe("invites.used");
    expect(result.current.details).toBeNull();
    expect(result.current.retryable).toBe(false);
    expect(opened).not.toHaveBeenCalled();
  });

  it("opens the org when the caller joined it since the preview", async () => {
    mocks.mutate.mockRejectedValueOnce(graphQLError("INVITE_ALREADY_ACCEPTED"));
    mocks.previewRefetch.mockResolvedValueOnce({
      data: {
        organizationInvitePreview: {
          organizationId: "org-target",
          alreadyMember: true,
        },
      },
    });
    const opened = vi.fn();
    const { result } = renderHook(() =>
      useOrganizationInviteRedemption(TOKEN, opened),
    );
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBeNull();
    expect(opened).toHaveBeenCalledWith("org-target");
  });

  it.each([
    ["INVITE_EXPIRED", "invites.orgExpired"],
    ["INVITE_INVALID", "invites.orgInvalid"],
  ])("closes the invitation on %s", async (code, key) => {
    mocks.mutate.mockRejectedValueOnce(graphQLError(code));
    const { result } = renderHook(() =>
      useOrganizationInviteRedemption(TOKEN, vi.fn()),
    );
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBe(key);
    expect(result.current.details).toBeNull();
    expect(result.current.retryable).toBe(false);
  });

  it("keeps the invitation for a retry after an ambiguous failure", async () => {
    mocks.mutate.mockRejectedValueOnce(new TypeError("offline"));
    const { result } = renderHook(() =>
      useOrganizationInviteRedemption(TOKEN, vi.fn()),
    );
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBe("invites.retryError");
    expect(result.current.details).not.toBeNull();
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(
      `org:${TOKEN}`,
    );
  });

  it("names the org refusal when the preview itself is refused", () => {
    mocks.preview.error = graphQLError("INVITE_ALREADY_ACCEPTED");
    const { result } = renderHook(() =>
      useOrganizationInviteRedemption(TOKEN, vi.fn()),
    );
    expect(result.current.errorKey).toBe("invites.used");
    expect(result.current.retryable).toBe(false);
  });
});
