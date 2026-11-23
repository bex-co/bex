import { buildLogQuery, hasStoreOnlyTailFilters } from "../query";

describe("log query", () => {
  it("preserves repeatable Render filters", () => {
    const query = buildLogQuery({
      resource: "srv-1",
      types: ["app", "request"],
      statusCode: ["404", "5xx"],
      method: ["GET"],
      text: "timeout & retry",
      limit: 500,
    });
    const params = new URLSearchParams(query);
    expect(params.getAll("resource")).toEqual(["srv-1"]);
    expect(params.getAll("type")).toEqual(["app", "request"]);
    expect(params.getAll("statusCode")).toEqual(["404", "5xx"]);
    expect(params.getAll("method")).toEqual(["GET"]);
    expect(params.get("text")).toBe("timeout & retry");
    expect(params.get("limit")).toBe("100");
  });

  it("preserves smaller page sizes and leaves the API default unset", () => {
    expect(
      new URLSearchParams(buildLogQuery({ resource: "srv-1", limit: 20 })).get(
        "limit",
      ),
    ).toBe("20");
    expect(
      new URLSearchParams(buildLogQuery({ resource: "srv-1" })).get("limit"),
    ).toBe(null);
  });

  it("sends textMode only when text is present and opted in", () => {
    const withDisplay = new URLSearchParams(
      buildLogQuery({
        resource: "srv-1",
        text: "==> Deploying...",
        textMode: "display",
      }),
    );
    expect(withDisplay.get("text")).toBe("==> Deploying...");
    expect(withDisplay.get("textMode")).toBe("display");

    const withoutText = new URLSearchParams(
      buildLogQuery({ resource: "srv-1", textMode: "display" }),
    );
    expect(withoutText.get("textMode")).toBe(null);

    const withoutMode = new URLSearchParams(
      buildLogQuery({ resource: "srv-1", text: "==> Deploying..." }),
    );
    expect(withoutMode.get("textMode")).toBe(null);
  });

  it("distinguishes pod-tail filters from store-only filters", () => {
    expect(
      hasStoreOnlyTailFilters({
        resource: "srv-1",
        types: ["app"],
        instance: ["web-abc"],
        text: "ready",
        textMode: "display",
      }),
    ).toBe(false);
    expect(
      hasStoreOnlyTailFilters({ resource: "srv-1", level: ["error"] }),
    ).toBe(true);
    expect(
      hasStoreOnlyTailFilters({ resource: "srv-1", types: ["request"] }),
    ).toBe(true);
  });
});
