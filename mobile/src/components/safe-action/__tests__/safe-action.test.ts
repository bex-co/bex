import { SafeActionExecutor, classifySafeActionFailure } from "../executor";
import { confirmSafeAction, createSafeActionIntent } from "../model";
import { defineSafeAction } from "../registry";

const restart = defineSafeAction("restart-service", "service");
const target = { kind: "service" as const, id: "srv-safe", label: "api" };
const identity = () => "retry-fixed";

describe("safe actions", () => {
  it("binds confirmation to the exact action and target", () => {
    const intent = createSafeActionIntent(restart, target, identity);
    expect(intent.confirmed).toBe(false);
    expect(confirmSafeAction(intent)).toEqual({ ...intent, confirmed: true });
    expect(() =>
      confirmSafeAction({ ...intent, confirmationKey: "stale-target" }),
    ).toThrow("safe action confirmation no longer matches its target");
    expect(() =>
      createSafeActionIntent(restart, {
        kind: "database",
        id: "dpg-wrong",
        label: "db",
      }),
    ).toThrow("mobile action restart-service cannot target database");
  });

  it("refuses execution without explicit confirmation", async () => {
    const executor = new SafeActionExecutor();
    const intent = createSafeActionIntent(restart, target, identity);
    let rejected = false;
    try {
      await executor.execute(intent, async () => ({ data: "never" }));
    } catch {
      rejected = true;
    }
    expect(rejected).toBe(true);
  });

  it("dedupes double taps by action and target while preserving retry identity", async () => {
    const executor = new SafeActionExecutor();
    const intent = confirmSafeAction(
      createSafeActionIntent(restart, target, identity),
    );
    let release = () => {};
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const identities: string[] = [];
    const operation = async ({ retryIdentity }: { retryIdentity: string }) => {
      identities.push(retryIdentity);
      await gate;
      return { data: "ok", audit: { status: "recorded" as const } };
    };

    const first = executor.execute(intent, operation);
    const second = executor.execute(intent, operation);
    expect(first === second).toBe(true);
    expect(executor.isPending(intent)).toBe(true);
    release();
    expect(await first).toEqual({
      status: "succeeded",
      feedback: "success",
      data: "ok",
      retryIdentity: "retry-fixed",
      auditStatus: "recorded",
      auditEventId: undefined,
      canRetry: false,
    });
    await Promise.resolve();
    expect(identities).toEqual(["retry-fixed"]);
    expect(executor.isPending(intent)).toBe(false);
  });

  it("keeps timeout-after-commit honest and requires reconciliation", async () => {
    const executor = new SafeActionExecutor();
    const intent = confirmSafeAction(
      createSafeActionIntent(restart, target, identity),
    );
    const timeout = Object.assign(new Error("gateway timeout"), {
      name: "TimeoutError",
    });
    const result = await executor.execute(intent, async () => {
      throw timeout;
    });
    expect(result.status).toBe("unknown");
    expect(result.feedback).toBe("timeout-unknown");
    expect(result.canRetry).toBe(false);
    expect(result.retryIdentity).toBe("retry-fixed");
  });

  it("reports committed audit trouble as partial and never as success", async () => {
    const executor = new SafeActionExecutor();
    const intent = confirmSafeAction(
      createSafeActionIntent(restart, target, identity),
    );
    const result = await executor.execute(intent, async () => ({
      data: "committed",
      audit: { status: "unavailable" },
    }));
    expect(result.status).toBe("partial");
    expect(result.feedback).toBe("audit-unavailable");
    expect(result.canRetry).toBe(false);
  });

  it("keeps accepted but unverifiable work distinct from success", async () => {
    const executor = new SafeActionExecutor();
    const intent = confirmSafeAction(
      createSafeActionIntent(restart, target, identity),
    );
    const result = await executor.execute(intent, async () => ({
      data: "accepted",
      feedback: "accepted-unverified",
    }));
    expect(result.status).toBe("partial");
    expect(result.feedback).toBe("accepted-unverified");
    expect(result.canRetry).toBe(false);
  });

  it("classifies nested authorization, conflict, timeout, and audit errors", () => {
    expect(
      classifySafeActionFailure({
        graphQLErrors: [{ extensions: { code: "FORBIDDEN" } }],
      }),
    ).toBe("authorization-denied");
    expect(classifySafeActionFailure({ statusCode: 409 })).toBe("conflict");
    expect(classifySafeActionFailure({ cause: { status: 504 } })).toBe(
      "timeout-unknown",
    );
    expect(classifySafeActionFailure({ code: "AUDIT_STORE_UNAVAILABLE" })).toBe(
      "audit-unavailable",
    );
    expect(classifySafeActionFailure(new Error("ordinary failure"))).toBe(
      "failed",
    );
    expect(classifySafeActionFailure(new Error("forbidden"))).toBe(
      "authorization-denied",
    );
    expect(classifySafeActionFailure(new Error("resource conflict"))).toBe(
      "conflict",
    );
  });

  it("cancels in-flight work without presenting a failure or success", async () => {
    const executor = new SafeActionExecutor();
    const intent = confirmSafeAction(
      createSafeActionIntent(restart, target, identity),
    );
    const pending = executor.execute(intent, async ({ signal }) => {
      await new Promise<void>((_resolve, reject) => {
        signal.addEventListener("abort", () => reject(new Error("aborted")), {
          once: true,
        });
      });
      return { data: "never" };
    });
    executor.cancel(intent);
    expect((await pending).status).toBe("canceled");
  });
});
