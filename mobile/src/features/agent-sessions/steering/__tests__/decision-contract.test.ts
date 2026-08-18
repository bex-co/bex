import {
  buildDecisionResponse,
  initialDecisionValues,
  parseDecisionContract,
} from "../decision-contract";

describe("ACP decision contracts", () => {
  it("parses safe fields and emits typed response values", () => {
    const parsed = parseDecisionContract(
      JSON.stringify({
        type: "object",
        required: ["target", "replicas"],
        properties: {
          target: {
            type: "string",
            title: "Target",
            oneOf: [
              { const: "staging", title: "Staging" },
              { const: "prod", title: "Production" },
            ],
            default: "staging",
          },
          replicas: { type: "integer", minimum: 1, maximum: 5 },
          notify: { type: "boolean", default: true },
          checks: {
            type: "array",
            items: { type: "string", enum: ["lint", "test"] },
          },
        },
      }),
    );
    expect(parsed.ok).toBe(true);
    if (!parsed.ok) return;
    expect(initialDecisionValues(parsed.fields)).toEqual({
      target: "staging",
      notify: true,
    });
    expect(
      buildDecisionResponse(parsed.fields, {
        target: "prod",
        replicas: "3",
        notify: false,
        checks: ["lint", "test"],
      }),
    ).toEqual({
      ok: true,
      value: {
        target: "prod",
        replicas: 3,
        notify: false,
        checks: ["lint", "test"],
      },
    });
  });

  it("fails closed on secret-shaped or malformed fields", () => {
    expect(
      parseDecisionContract(
        JSON.stringify({
          properties: { apiKey: { type: "string", title: "API key" } },
        }),
      ),
    ).toEqual({ ok: false, reason: "sensitive" });
    expect(parseDecisionContract("not-json")).toEqual({
      ok: false,
      reason: "invalid",
    });
  });

  it("rejects incomplete, out-of-range, and undeclared values", () => {
    const parsed = parseDecisionContract(
      JSON.stringify({
        required: ["count"],
        properties: { count: { type: "integer", minimum: 1, maximum: 2 } },
      }),
    );
    expect(parsed.ok).toBe(true);
    if (!parsed.ok) return;
    expect(buildDecisionResponse(parsed.fields, { count: "3" })).toEqual({
      ok: false,
    });
    expect(buildDecisionResponse(parsed.fields, {})).toEqual({ ok: false });
  });
});
