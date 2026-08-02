import {
  DeployMutationFailure,
  mapDeployActionError,
} from "../deploy-action-errors";

describe("deploy action error mapping", () => {
  it("maps server conflict and authorization errors as deterministic refusals", () => {
    const conflict = mapDeployActionError({
      statusCode: 409,
      message: "deploy is already live",
    });
    expect(conflict.code).toBe("conflict");
    expect(conflict.delivery).toBe("rejected_by_server");
    expect(conflict.refreshRequired).toBe(true);

    const forbidden = mapDeployActionError(new Error("forbidden"));
    expect(forbidden.code).toBe("forbidden");
    expect(forbidden.retry).toBe("none");
  });

  it("treats a timeout after send as possibly committed and never safe to retry", () => {
    const mapped = mapDeployActionError(
      new DeployMutationFailure(
        "mutation timed out",
        "possibly_sent",
        Object.assign(new Error("request timeout"), { name: "AbortError" }),
      ),
    );
    expect(mapped.code).toBe("timeout");
    expect(mapped.delivery).toBe("possibly_committed");
    expect(mapped.refreshRequired).toBe(true);
    expect(mapped.retry).toBe("after_refresh");
  });

  it("allows a new confirmation only when the transport proves nothing was sent", () => {
    const mapped = mapDeployActionError(
      new DeployMutationFailure("offline", "not_sent", new Error("offline")),
    );
    expect(mapped.code).toBe("network");
    expect(mapped.delivery).toBe("not_sent");
    expect(mapped.retry).toBe("safe");
  });
});
