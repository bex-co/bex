import { describe, expect, it } from "vitest";
import { Route } from "../databases.$databaseId";

describe("database detail tab search contract", () => {
  const validate = Route.options.validateSearch as (
    search: Record<string, unknown>,
  ) => { tab?: "logs" };

  it("keeps the directly linkable Logs tab and drops unknown tab values", () => {
    expect(validate({ tab: "logs" })).toEqual({ tab: "logs" });
    expect(validate({ tab: "metrics" })).toEqual({});
    expect(validate({})).toEqual({});
  });
});
