import { describe, expect, it } from "vitest";
import { SavedSearchType } from "@/graphql/definitions";
import {
  VIEW_FIELDS_BY_TYPE,
  documentFromRows,
  freeViewName,
  incompleteRows,
  newFilterRow,
  rowsFromDocument,
  viewFieldIsSingle,
  viewFieldOperators,
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
        { field: "LEVEL", operator: "IS", values: ["error", "warning"] },
      ],
      version: 1,
    };
    const rows = rowsFromDocument(doc, SavedSearchType.ApplicationLogs);
    expect(rows.map((row) => row.field)).toEqual([
      "ENVIRONMENT",
      "RESOURCE_TYPE",
      "LEVEL",
    ]);
    expect(new Set(rows.map((row) => row.key)).size).toBe(3);
    expect(documentFromRows(rows)).toEqual(doc);
  });

  it("keeps only what this view type's editor can show", () => {
    const rows = rowsFromDocument(
      {
        filters: [
          { field: "METHOD", operator: "IS", values: ["GET"] },
          { field: "LEVEL", operator: "MATCHES", values: ["error"] },
          { field: "BOGUS", operator: "IS", values: ["x"] },
          { field: "TEXT", operator: "IS", values: ["timeout", 7] },
          "not a filter",
        ],
        version: 1,
      },
      SavedSearchType.ApplicationLogs,
    );
    expect(rows).toEqual([
      {
        key: expect.any(String),
        field: "TEXT",
        operator: "IS",
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
      "TEXT",
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

  it("negates only the scope fields bex-api negates", () => {
    expect(viewFieldOperators("RESOURCE")).toEqual(["IS", "IS_NOT"]);
    expect(viewFieldOperators("RESOURCE_TYPE")).toEqual(["IS", "IS_NOT"]);
    // Render's editor: Environment takes `is`; the line fields wait on w3/076.
    for (const field of ["ENVIRONMENT", "LEVEL", "TEXT", "PATH"] as const) {
      expect(viewFieldOperators(field)).toEqual(["IS"]);
    }
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
