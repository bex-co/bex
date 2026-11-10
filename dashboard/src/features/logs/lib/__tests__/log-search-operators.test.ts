import { describe, expect, it } from "vitest";
import { parseSearchOperators } from "@/features/logs/lib/log-search-operators";

// w4/538: Render's log search field operators (render.com/docs/logging).
describe("parseSearchOperators", () => {
  it("lifts each documented operator into its filter", () => {
    expect(
      parseSearchOperators(
        "level:error instance:web-1 method:/(GET|POST)/ status_code:/4../ host:a.onbex.co path:api/resource/*/sub",
      ),
    ).toEqual({
      text: "",
      filters: {
        level: "error",
        instance: "web-1",
        method: "/(GET|POST)/",
        statusCode: "/4../",
        host: "a.onbex.co",
        path: "api/resource/*/sub",
      },
    });
  });

  it("keeps free text, unknown names and empty values as text", () => {
    expect(
      parseSearchOperators("  boom  color:red  level:  STATUS_CODE:500 "),
    ).toEqual({
      text: "boom color:red level:",
      filters: { statusCode: "500" },
    });
    expect(parseSearchOperators("plain words")).toEqual({
      text: "plain words",
      filters: {},
    });
  });
});
