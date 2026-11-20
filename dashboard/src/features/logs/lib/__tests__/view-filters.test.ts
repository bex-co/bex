import { describe, expect, it } from "vitest";
import { SavedSearchType } from "@/graphql/definitions";
import {
  VIEW_FIELDS_BY_TYPE,
  documentFromRows,
  freeViewName,
  incompleteRows,
  instanceChoiceLabel,
  newFilterRow,
  rowsFromDocument,
  viewFieldIsSingle,
  viewFieldLogLabel,
  viewFieldOperators,
  viewLogType,
  viewValueKind,
  withField,
} from "../view-filters";

// The Edit view modal's model of Render's filter document (w3/m507): what it
// reads from a stored view, what it saves, and the rules it shares with
// bex-api's internal/savedsearch validator.

describe("the view filter document", () => {
  it("round-trips a stored document through the editor's rows", () => {
    const doc = {
      filters: [
        { field: "ENVIRONMENT", operator: "IS", values: ["evm-1"] },
        { field: "RESOURCE_TYPE", operator: "IS_NOT", values: ["POSTGRES"] },
        { field: "LEVEL", operator: "IS_NOT", values: ["info", "debug"] },
      ],
      version: 1,
    };
    const rows = rowsFromDocument(doc, SavedSearchType.ApplicationLogs);
    expect(rows[2]).toMatchObject({ field: "LEVEL", operator: "IS_NOT" });
    expect(rows.map((row) => row.field)).toEqual([
      "ENVIRONMENT",
      "RESOURCE_TYPE",
      "LEVEL",
    ]);
    expect(new Set(rows.map((row) => row.key)).size).toBe(3);
    expect(documentFromRows(rows)).toEqual(doc);
  });

  it("reads and saves Render's document, its range and display (w3/083)", () => {
    const doc = {
      filters: [
        { field: "LEVEL", operator: "IS", values: ["error"] },
        { field: "TEXT_SEARCH", operator: "CONTAINS", values: ["timeout"] },
      ],
      version: 1,
      timeRange: { relative: "4h" },
      display: { aggregationMethod: "AVG", requestGroupBy: "STATUS_CODE" },
    };
    const rows = rowsFromDocument(doc, SavedSearchType.ApplicationLogs);
    expect(
      rows.map(({ field, operator, values }) => [field, operator, values]),
    ).toEqual([
      ["LEVEL", "IS", ["error"]],
      ["TEXT_SEARCH", "CONTAINS", ["timeout"]],
      ["TIME_RANGE", "IS", ["4h"]],
    ]);
    expect(documentFromRows(rows, doc)).toEqual(doc);
    // A view with no range saves none, and a new view has no display.
    expect(documentFromRows(rows.slice(0, 1))).toEqual({
      filters: [{ field: "LEVEL", operator: "IS", values: ["error"] }],
      version: 1,
    });
  });

  it("reads bex's earlier document and saves it in Render's shape", () => {
    const rows = rowsFromDocument(
      {
        filters: [
          { field: "TIME_RANGE", operator: "IS", values: ["7d"] },
          { field: "TEXT", operator: "IS", values: ["timeout"] },
        ],
        version: 1,
      },
      SavedSearchType.ApplicationLogs,
    );
    expect(
      rows.map(({ field, operator, values }) => [field, operator, values]),
    ).toEqual([
      ["TEXT_SEARCH", "CONTAINS", ["timeout"]],
      ["TIME_RANGE", "IS", ["7d"]],
    ]);
    expect(documentFromRows(rows)).toEqual({
      filters: [
        { field: "TEXT_SEARCH", operator: "CONTAINS", values: ["timeout"] },
      ],
      version: 1,
      timeRange: { relative: "7d" },
    });
  });

  it("keeps only what this view type's editor can show", () => {
    const rows = rowsFromDocument(
      {
        filters: [
          { field: "METHOD", operator: "IS", values: ["GET"] },
          { field: "LEVEL", operator: "MATCHES", values: ["error"] },
          { field: "LEVEL", operator: "CONTAINS", values: ["error"] },
          { field: "BOGUS", operator: "IS", values: ["x"] },
          { field: "TEXT_SEARCH", operator: "IS", values: ["x"] },
          {
            field: "TEXT_SEARCH",
            operator: "CONTAINS",
            values: ["timeout", 7],
          },
          "not a filter",
        ],
        version: 1,
        timeRange: { relative: 4 },
      },
      SavedSearchType.ApplicationLogs,
    );
    expect(rows).toEqual([
      {
        key: expect.any(String),
        field: "TEXT_SEARCH",
        operator: "CONTAINS",
        values: ["timeout"],
      },
    ]);
    expect(rowsFromDocument(undefined, SavedSearchType.RequestLogs)).toEqual(
      [],
    );
    expect(
      rowsFromDocument({ filters: "no" }, SavedSearchType.RequestLogs),
    ).toEqual([]);
  });

  it("offers the capture's fields per type, in Render's order", () => {
    expect(VIEW_FIELDS_BY_TYPE[SavedSearchType.ApplicationLogs]).toEqual([
      "ENVIRONMENT",
      "RESOURCE",
      "RESOURCE_TYPE",
      "LEVEL",
      "INSTANCE",
      "TEXT_SEARCH",
      "TIME_RANGE",
    ]);
    expect(VIEW_FIELDS_BY_TYPE[SavedSearchType.RequestLogs]).toEqual([
      "ENVIRONMENT",
      "RESOURCE",
      "RESOURCE_TYPE",
      "LEVEL",
      "INSTANCE",
      "METHOD",
      "STATUS_CODE",
      "HOST",
      "PATH",
      "TIME_RANGE",
    ]);
  });

  it("negates every field Render's editor negates", () => {
    // Render's Edit view: `is one of` / `is not one of` on the scope fields
    // and the line fields, `is` on Environment and Time Range, and
    // `contains` on Text Search (w3/m523, w3/083).
    for (const field of [
      "RESOURCE",
      "RESOURCE_TYPE",
      "LEVEL",
      "INSTANCE",
      "METHOD",
      "STATUS_CODE",
      "HOST",
      "PATH",
    ] as const) {
      expect(viewFieldOperators(field)).toEqual(["IS", "IS_NOT"]);
    }
    for (const field of ["ENVIRONMENT", "TIME_RANGE"] as const) {
      expect(viewFieldOperators(field)).toEqual(["IS"]);
    }
    expect(viewFieldOperators("TEXT_SEARCH")).toEqual(["CONTAINS"]);
    expect(viewFieldIsSingle("ENVIRONMENT")).toBe(true);
    expect(viewFieldIsSingle("TIME_RANGE")).toBe(true);
    expect(viewFieldIsSingle("RESOURCE")).toBe(false);
  });

  it("adds a row on the first unused field, and none once every field is used", () => {
    const type = SavedSearchType.ApplicationLogs;
    const rows = [newFilterRow(type, [])!];
    expect(rows[0]).toMatchObject({
      field: "ENVIRONMENT",
      operator: "IS",
      values: [],
    });
    while (true) {
      const next = newFilterRow(type, rows);
      if (!next) break;
      rows.push(next);
    }
    expect(rows.map((row) => row.field)).toEqual(VIEW_FIELDS_BY_TYPE[type]);
    expect(incompleteRows(rows).size).toBe(rows.length);

    const moved = withField(
      { ...rows[1], operator: "IS_NOT", values: ["srv-1"] },
      "LEVEL",
    );
    expect(moved).toMatchObject({ field: "LEVEL", operator: "IS", values: [] });
    expect(moved.key).toBe(rows[1].key);
  });

  it("picks Instance from the project's logs and names each by its resource (w3/079)", () => {
    expect(viewValueKind("INSTANCE")).toBe("discovered");
    expect(viewFieldLogLabel("INSTANCE")).toBe("instance");
    expect(viewFieldLogLabel("TEXT_SEARCH")).toBeUndefined();
    expect(viewLogType(SavedSearchType.ApplicationLogs)).toBe("app");
    expect(viewLogType(SavedSearchType.RequestLogs)).toBe("request");

    const resources = [
      { value: "srv-web", label: "storefront" },
      { value: "srv-web2", label: "storefront-2" },
    ];
    expect(instanceChoiceLabel("srv-web-0123456789abcdefghij", resources)).toBe(
      "storefront · 01234",
    );
    // srv-web's id begins srv-web2's, but only the whole id and its dash
    // name an instance's resource.
    expect(
      instanceChoiceLabel("srv-web2-0123456789abcdefghij", resources),
    ).toBe("storefront-2 · 01234");
    expect(
      instanceChoiceLabel("srv-gone-0123456789abcdefghij", resources),
    ).toBe("srv-gone-0123456789abcdefghij");
  });

  it("numbers a new view's name when the project already has it", () => {
    expect(freeViewName("Request Logs", ["Application Logs"])).toBe(
      "Request Logs",
    );
    expect(
      freeViewName("Application Logs", [
        "Application Logs",
        "Application Logs 2",
      ]),
    ).toBe("Application Logs 3");
  });
});
