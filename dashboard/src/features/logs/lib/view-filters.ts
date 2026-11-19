import type { en } from "@/i18n";
import { SavedSearchType } from "@/graphql/definitions";
import { LOG_LEVELS, LOG_METHODS, STRUCTURED_LABEL_KEYS } from "../types";

/**
 * The Edit view modal's model of a saved search's filter document (w3/m507):
 * Render's `{filters: [{field, operator, values}], version: 1}`, edited as
 * rows. bex-api's `internal/savedsearch` validates every document it stores;
 * these tables are the editor's half of the same rules, so it only offers
 * what the server takes.
 */

export const VIEW_FILTER_VERSION = 1;

export type ViewOperator = "IS" | "IS_NOT";

export const VIEW_FIELDS = [
  "ENVIRONMENT",
  "RESOURCE",
  "RESOURCE_TYPE",
  "LEVEL",
  "INSTANCE",
  "TEXT",
  "METHOD",
  "STATUS_CODE",
  "HOST",
  "PATH",
  "TIME_RANGE",
] as const;
export type ViewField = (typeof VIEW_FIELDS)[number];

/** Which fields each view type's editor offers, in Render's order (the capture's table). */
export const VIEW_FIELDS_BY_TYPE: Record<SavedSearchType, ViewField[]> = {
  [SavedSearchType.ApplicationLogs]: [
    "ENVIRONMENT",
    "RESOURCE",
    "RESOURCE_TYPE",
    "LEVEL",
    "INSTANCE",
    "TEXT",
    "TIME_RANGE",
  ],
  [SavedSearchType.RequestLogs]: [
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
  ],
  [SavedSearchType.Metrics]: [
    "ENVIRONMENT",
    "RESOURCE",
    "RESOURCE_TYPE",
    "TIME_RANGE",
  ],
};

export const VIEW_FIELD_LABEL_KEYS: Record<ViewField, keyof typeof en> = {
  ENVIRONMENT: "logs.viewFieldEnvironment",
  RESOURCE: "logs.viewFieldResource",
  RESOURCE_TYPE: "logs.viewFieldResourceType",
  LEVEL: STRUCTURED_LABEL_KEYS.level,
  INSTANCE: STRUCTURED_LABEL_KEYS.instance,
  TEXT: "logs.viewFieldText",
  METHOD: STRUCTURED_LABEL_KEYS.method,
  STATUS_CODE: STRUCTURED_LABEL_KEYS.statusCode,
  HOST: STRUCTURED_LABEL_KEYS.host,
  PATH: STRUCTURED_LABEL_KEYS.path,
  TIME_RANGE: "logs.viewFieldTimeRange",
};

/** Render's Resource Type values, each with its label. */
const VIEW_RESOURCE_TYPE_OPTIONS: ViewOption[] = [
  { value: "SERVICE", labelKey: "logs.viewResourceTypeService" },
  { value: "CRON_JOB", labelKey: "logs.viewResourceTypeCronJob" },
  { value: "POSTGRES", labelKey: "logs.viewResourceTypePostgres" },
  { value: "KEY_VALUE", labelKey: "logs.viewResourceTypeKeyValue" },
];

/** One fixed value of a field: shown as itself, or by its label. */
export interface ViewOption {
  value: string;
  labelKey?: keyof typeof en;
}

/**
 * How a field's values are picked: from the project's environments or
 * resources, from a fixed list, or typed (an instance, a search term, a
 * status code, a host or a path).
 */
export type ViewValueKind =
  "environment" | "resource" | "options" | "timeRange" | "text";

export function viewValueKind(field: ViewField): ViewValueKind {
  switch (field) {
    case "ENVIRONMENT":
      return "environment";
    case "RESOURCE":
      return "resource";
    case "RESOURCE_TYPE":
    case "LEVEL":
    case "METHOD":
      return "options";
    case "TIME_RANGE":
      return "timeRange";
    default:
      return "text";
  }
}

/** The fixed values of an `options` field. */
export function viewFieldOptions(field: ViewField): readonly ViewOption[] {
  switch (field) {
    case "RESOURCE_TYPE":
      return VIEW_RESOURCE_TYPE_OPTIONS;
    case "LEVEL":
      return LOG_LEVELS.map((value) => ({ value }));
    case "METHOD":
      return LOG_METHODS.map((value) => ({ value }));
    default:
      return [];
  }
}

/** A field that takes exactly one value: an environment, a time range. */
export function viewFieldIsSingle(field: ViewField): boolean {
  return field === "ENVIRONMENT" || field === "TIME_RANGE";
}

/**
 * The operators a field takes. `ENVIRONMENT` takes `is`, as Render's editor
 * does; Resource and Resource Type take `is one of` / `is not one of`. The
 * line fields take `is` only: the log query has no negation yet (w3/076), so
 * bex-api refuses `IS_NOT` on them by name.
 */
export function viewFieldOperators(field: ViewField): ViewOperator[] {
  return field === "RESOURCE" || field === "RESOURCE_TYPE"
    ? ["IS", "IS_NOT"]
    : ["IS"];
}

/** One condition row in the editor. `key` keys the row in the list only. */
export interface ViewFilterRow {
  key: string;
  field: ViewField;
  operator: ViewOperator;
  values: string[];
}

export interface ViewFilterDocument extends Record<string, unknown> {
  filters: { field: ViewField; operator: ViewOperator; values: string[] }[];
  version: number;
}

let rowCounter = 0;
function rowKey(): string {
  rowCounter += 1;
  return `filter-${rowCounter}`;
}

function isViewField(value: unknown): value is ViewField {
  return (VIEW_FIELDS as readonly unknown[]).includes(value);
}

/**
 * The rows of a stored document, keeping only what the editor can show: a
 * field of this view's type, a known operator and string values. bex-api
 * validated the document when it stored it, so nothing is normally dropped.
 */
export function rowsFromDocument(
  doc: Record<string, unknown> | undefined,
  type: SavedSearchType,
): ViewFilterRow[] {
  const filters = doc?.filters;
  if (!Array.isArray(filters)) return [];
  const allowed = VIEW_FIELDS_BY_TYPE[type];
  const rows: ViewFilterRow[] = [];
  for (const raw of filters) {
    if (!raw || typeof raw !== "object") continue;
    const { field, operator, values } = raw as Record<string, unknown>;
    if (!isViewField(field) || !allowed.includes(field)) continue;
    if (operator !== "IS" && operator !== "IS_NOT") continue;
    rows.push({
      key: rowKey(),
      field,
      operator,
      values: Array.isArray(values)
        ? values.filter((v): v is string => typeof v === "string")
        : [],
    });
  }
  return rows;
}

/** The document a set of rows saves as. */
export function documentFromRows(rows: ViewFilterRow[]): ViewFilterDocument {
  return {
    filters: rows.map(({ field, operator, values }) => ({
      field,
      operator,
      values,
    })),
    version: VIEW_FILTER_VERSION,
  };
}

/**
 * A new row: the first of the type's fields no row uses yet, with its first
 * operator and no values. Null when every field is in use (a field appears at
 * most once in a document).
 */
export function newFilterRow(
  type: SavedSearchType,
  rows: ViewFilterRow[],
): ViewFilterRow | null {
  const used = new Set(rows.map((row) => row.field));
  const field = VIEW_FIELDS_BY_TYPE[type].find((f) => !used.has(f));
  if (!field) return null;
  return {
    key: rowKey(),
    field,
    operator: viewFieldOperators(field)[0],
    values: [],
  };
}

/** Whether a field of the type is left for another row. */
export function canAddFilterRow(
  type: SavedSearchType,
  rows: ViewFilterRow[],
): boolean {
  return rows.length < VIEW_FIELDS_BY_TYPE[type].length;
}

/** A row moved to another field: the field's first operator, no values. */
export function withField(row: ViewFilterRow, field: ViewField): ViewFilterRow {
  return { ...row, field, operator: viewFieldOperators(field)[0], values: [] };
}

/** The rows that cannot be saved: a row with no value. */
export function incompleteRows(rows: ViewFilterRow[]): Set<string> {
  return new Set(
    rows.filter((row) => row.values.length === 0).map((row) => row.key),
  );
}

/**
 * The name a new view of a type starts with: the type's own name, numbered
 * when the project already has a view called that ("Application Logs 2"), so
 * Done does not meet `duplicate_saved_search_name` unless the user picks a
 * taken name.
 */
export function freeViewName(base: string, taken: readonly string[]): string {
  const names = new Set(taken);
  if (!names.has(base)) return base;
  for (let n = 2; ; n += 1) {
    const candidate = `${base} ${n}`;
    if (!names.has(candidate)) return candidate;
  }
}
