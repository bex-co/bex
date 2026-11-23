import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";

const runCronJob = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [runCronJob],
}));

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
}));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { error: (...a: unknown[]) => toastError(...a) },
}));

vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({ t: (k: string) => k }),
}));

import { useTriggerCronRun } from "@/features/services/hooks/use-trigger-cron-run";

const KEPT = { duration: Infinity, closeButton: true };

beforeEach(() => {
  runCronJob.mockReset();
  navigate.mockReset().mockResolvedValue(undefined);
  toastError.mockReset();
});

async function trigger() {
  const { result } = renderHook(() => useTriggerCronRun("nightly"));
  await act(async () => {
    await result.current.trigger();
  });
  return result;
}

// w3/m555: Render's Trigger Run fires runCronJob with no confirm, then opens
// the cron job's Logs on Live tail (`logs?r=live`; bex's `range=live`, w3/m537).
describe("useTriggerCronRun", () => {
  it("runs at once and opens the Logs on Live tail", async () => {
    runCronJob.mockResolvedValue({
      data: { runCronJob: { id: "crr-new", status: "pending" } },
    });
    const result = await trigger();

    expect(runCronJob).toHaveBeenCalledWith({ variables: { id: "nightly" } });
    expect(navigate).toHaveBeenCalledOnce();
    expect(navigate).toHaveBeenCalledWith({
      to: "/services/$serviceId/logs",
      params: { serviceId: "nightly" },
      search: expect.objectContaining({ range: "live" }),
    });
    expect(toastError).not.toHaveBeenCalled();
    expect(result.current.triggering).toBe(false);
  });

  it("opens Events when the answer carries no run, as Render's does", async () => {
    runCronJob.mockResolvedValue({ data: { runCronJob: null } });
    await trigger();
    expect(navigate).toHaveBeenCalledWith({
      to: "/services/$serviceId/events",
      params: { serviceId: "nightly" },
    });
  });

  it.each([
    [
      "a permission refusal in Render's words",
      codedGraphQLError("FORBIDDEN"),
      "services.cronTriggerForbidden",
    ],
    [
      "a suspended cron job as its localized reason, never the server's text (w4/203)",
      codedGraphQLError("CRON_SUSPENDED"),
      "capabilities.blockedSuspended",
    ],
    [
      "any other refusal in the server's own words",
      uncodedGraphQLError("billing hold on this workspace"),
      "Billing hold on this workspace",
    ],
    [
      "a transport failure in Render's fallback words",
      new TypeError("Failed to fetch"),
      "services.cronTriggerError",
    ],
  ])(
    "toasts %s until dismissed, and stays on the page",
    async (_, error, text) => {
      runCronJob.mockRejectedValue(error);
      const result = await trigger();
      expect(toastError).toHaveBeenCalledWith(text, KEPT);
      expect(navigate).not.toHaveBeenCalled();
      expect(result.current.triggering).toBe(false);
    },
  );

  it("reports triggering while the mutation is out", async () => {
    let answer!: (value: unknown) => void;
    runCronJob.mockReturnValue(new Promise((resolve) => (answer = resolve)));
    const { result } = renderHook(() => useTriggerCronRun("nightly"));
    let done!: Promise<void>;
    act(() => {
      done = result.current.trigger();
    });
    expect(result.current.triggering).toBe(true);
    await act(async () => {
      answer({ data: { runCronJob: { id: "crr-new" } } });
      await done;
    });
    expect(result.current.triggering).toBe(false);
  });
});
