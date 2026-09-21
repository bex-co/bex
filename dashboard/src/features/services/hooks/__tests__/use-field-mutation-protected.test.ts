import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
  },
}));

const ask = vi.fn();
vi.mock("@/common/providers/protected-retry-context", () => ({
  useAskForProtectedConfirmation: () => ask,
}));

import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";
import { SetImageDocument } from "@/graphql/definitions";

const keys = { success: "services.sourceUpdateSuccess", error: "x" };

// The refusal bex-api returns for a protected member, verbatim in shape: the
// dashboard must read the phrase out of it rather than rebuild it (ADR032).
const REFUSAL = new Error(
  '"web" is a member of a protected environment; retry with confirm="sudo repoint service web" to repoint it',
);

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  ask.mockReset();
});

function setup(mutate: ReturnType<typeof vi.fn>) {
  mockUseMutation.mockReturnValue([mutate]);
  return renderHook(() =>
    useFieldMutation(
      SetImageDocument,
      (id: string, image: string) => ({ id, image }),
      keys,
    ),
  );
}

// w4/m126: every single-field Settings save inherits the protected-environment
// handshake from this one hook, so the handshake is pinned here.
describe("useFieldMutation protected-environment retry", () => {
  it("asks for the server's phrase and retries the mutation with it", async () => {
    const mutate = vi
      .fn()
      .mockRejectedValueOnce(REFUSAL)
      .mockResolvedValueOnce({});
    ask.mockResolvedValue("sudo repoint service web");
    const { result } = setup(mutate);

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.run("srv-1", "nginx:1.27");
    });

    expect(ok).toBe(true);
    // The phrase handed to the dialog is the server's, character for character.
    expect(ask).toHaveBeenCalledWith("sudo repoint service web");
    expect(mutate).toHaveBeenNthCalledWith(2, {
      variables: {
        id: "srv-1",
        image: "nginx:1.27",
        confirm: "sudo repoint service web",
      },
    });
    expect(toastSuccess).toHaveBeenCalledTimes(1);
    expect(toastError).not.toHaveBeenCalled();
  });

  it("stays silent and does not retry when the user dismisses the dialog", async () => {
    const mutate = vi.fn().mockRejectedValue(REFUSAL);
    ask.mockResolvedValue(null);
    const { result } = setup(mutate);

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.run("srv-1", "nginx:1.27");
    });

    expect(ok).toBe(false);
    expect(mutate).toHaveBeenCalledTimes(1);
    // Cancelling is a decision, not a failure — no error toast for it.
    expect(toastError).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("does not open the dialog for an ordinary error", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("image not found"));
    const { result } = setup(mutate);

    await act(async () => {
      await result.current.run("srv-1", "nope:1");
    });

    expect(ask).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledTimes(1);
  });

  it("reports a retry that is still refused", async () => {
    const mutate = vi.fn().mockRejectedValue(REFUSAL);
    ask.mockResolvedValue("sudo repoint service wrong");
    const { result } = setup(mutate);

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.run("srv-1", "nginx:1.27");
    });

    expect(ok).toBe(false);
    expect(mutate).toHaveBeenCalledTimes(2);
    expect(toastError).toHaveBeenCalledTimes(1);
  });

  it("clears busy after a dismissed confirmation", async () => {
    const mutate = vi.fn().mockRejectedValue(REFUSAL);
    ask.mockResolvedValue(null);
    const { result } = setup(mutate);

    await act(async () => {
      await result.current.run("srv-1", "nginx:1.27");
    });

    expect(result.current.busy).toBe(false);
  });
});
