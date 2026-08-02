import { classifyEnvironmentFailure } from "../environment-errors";

describe("classifyEnvironmentFailure", () => {
  it("uses stable revision codes for conflict and malformed tokens", () => {
    expect(
      classifyEnvironmentFailure({
        graphQLErrors: [
          { extensions: { code: "ENVIRONMENT_REVISION_CONFLICT" } },
        ],
      }),
    ).toBe("revision-conflict");
    expect(
      classifyEnvironmentFailure({
        extensions: { code: "ENVIRONMENT_REVISION_INVALID" },
      }),
    ).toBe("revision-invalid");
  });

  it("keeps denial, source, projection, compensation, and timeout distinct", () => {
    expect(classifyEnvironmentFailure({ statusCode: 403 })).toBe(
      "authorization-denied",
    );
    expect(classifyEnvironmentFailure({ code: "SECRETS_UNAVAILABLE" })).toBe(
      "secrets-unavailable",
    );
    expect(classifyEnvironmentFailure({ code: "SOURCE_WRITE_FAILED" })).toBe(
      "source-failed",
    );
    expect(classifyEnvironmentFailure({ code: "PROJECTION_FAILED" })).toBe(
      "projection-failed",
    );
    expect(classifyEnvironmentFailure({ code: "COMPENSATION_FAILED" })).toBe(
      "compensation-failed",
    );
    expect(
      classifyEnvironmentFailure({
        status: 409,
        code: "ENVIRONMENT_UPDATE_RESTORED",
      }),
    ).toBe("update-restored");
    expect(
      classifyEnvironmentFailure({
        status: 409,
        code: "ENVIRONMENT_RESTORATION_FAILED",
      }),
    ).toBe("compensation-failed");
    expect(classifyEnvironmentFailure({ name: "TimeoutError" })).toBe(
      "timeout-unknown",
    );
  });

  it("treats native network loss and gateway failures as ambiguous", () => {
    expect(
      classifyEnvironmentFailure(new TypeError("Network request failed")),
    ).toBe("timeout-unknown");
    expect(classifyEnvironmentFailure({ statusCode: 502 })).toBe(
      "timeout-unknown",
    );
    expect(classifyEnvironmentFailure({ status: 503 })).toBe("timeout-unknown");
  });
});
