import type { en } from "@/i18n";
import { SavedSearchType } from "@/graphql/definitions";
import {
  LOG_TYPE_APP,
  LOG_TYPE_REQUEST,
  STRUCTURED_LABEL_KEYS,
} from "../types";
import { shortInstance } from "./log-rows";

/**
 * The Edit view modal's model of a saved search's filter document (w3/m507):
 * Render's `{version: 1, filters: [{field, operator, values}], timeRange:
 * {relative}, display}` (w3/083), edited as rows, the time range as one of
 * them. bex-api's `internal/savedsearch` validates every document it stores;
 * these tables are the editor's half of the same rules, so it only offers
 * what the server takes.
 */

export const VIEW_FILTER_VERSION = 1;

export type ViewOperator = "IS" | "IS_NOT" | "CONTAINS";

export const VIEW_FIELDS = [
  "ENVIRONMENT",
  "RESOURCE",
  "RESOURCE_TYPE",
  "LEVEL",
  "INSTANCE",
  "TEXT_SEARCH",
  "METHOD",
  "STATUS_CODE",
  "HOST",
  "PATH",
  "TIME_RANGE",
] as const;
/** A row's field. `TIME_RANGE` is the editor's row for the document's
 *  top-level `timeRange`, never a filter of its own. */
export type ViewField = (typeof VIEW_FIELDS)[number];
type FilterField = Exclude<ViewField, "TIME_RANGE">;

/** Which fields each view type's editor offers, in Render's order (the capture's table). */
export const VIEW_FIELDS_BY_TYPE: Record<SavedSearchType, ViewField[]> = {
  [SavedSearchType.ApplicationLogs]: [
    "ENVIRONMENT",
    "RESOURCE",
    "RESOURCE_TYPE",
    "LEVEL",
    "INSTANCE",
    "TEXT_SEARCH",
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
  TEXT_SEARCH: "logs.viewFieldText",
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
 * resources, from Resource Type's fixed list, from the values the project's
 * logs hold (a level, an instance, a method, a status code, a host or a path),
 * or typed (a search term).
 */
export type ViewValueKind =
  "environment" | "resource" | "options" | "discovered" | "timeRange" | "text";

/**
 * The log label a `discovered` field's values are read from, across the
 * project's view (w3/079, w3/084), as Render's editor reads `logValues` for
 * each line field.
 */
const VIEW_FIELD_LOG_LABELS: Partial<Record<ViewField, string>> = {
  LEVEL: "level",
  INSTANCE: "instance",
  METHOD: "method",
  STATUS_CODE: "statusCode",
  HOST: "host",
  PATH: "path",
};

export function viewValueKind(field: ViewField): ViewValueKind {
  if (VIEW_FIELD_LOG_LABELS[field]) return "discovered";
  switch (field) {
    case "ENVIRONMENT":
      return "environment";
    case "RESOURCE":
      return "resource";
    case "RESOURCE_TYPE":
      return "options";
    case "TIME_RANGE":
      return "timeRange";
    default:
      return "text";
  }
}

/** The log label a `discovered` field reads its values from. */
export function viewFieldLogLabel(field: ViewField): string | undefined {
  return VIEW_FIELD_LOG_LABELS[field];
}

/** The log type a view of this type reads: its lines' `type`. */
export function viewLogType(
  type: SavedSearchType,
): typeof LOG_TYPE_APP | typeof LOG_TYPE_REQUEST {
  return type === SavedSearchType.RequestLogs ? LOG_TYPE_REQUEST : LOG_TYPE_APP;
}

/**
 * How the Instance picker names an instance: its resource's name and the
 * short instance a log line shows (`storefront · 01234`), when its id starts
 * with one of the resources' ids; otherwise the id itself.
 */
export function instanceChoiceLabel(
  instance: string,
  resources: readonly { value: string; label: string }[],
): string {
  const owner = resources.find((r) => instance.startsWith(`${r.value}-`));
  return owner ? `${owner.label} · ${shortInstance(instance)}` : instance;
}

/** The fixed values of an `options` field: Resource Type's. */
export function viewFieldOptions(field: ViewField): readonly ViewOption[] {
  return field === "RESOURCE_TYPE" ? VIEW_RESOURCE_TYPE_OPTIONS : [];
}

/** A field that takes exactly one value: an environment, a time range. */
export function viewFieldIsSingle(field: ViewField): boolean {
  return field === "ENVIRONMENT" || field === "TIME_RANGE";
}

/**
 * The operators a field takes, as Render's Edit view offers them (w3/m523):
 * Environment and Time Range take `is` and one value, Text Search takes
 * `contains`, and every other field, Level, Instance, Method, Status Code,
 * Host and Path as well as Resource and Resource Type, takes `is one of` /
 * `is not one of`.
 */
export function viewFieldOperators(field: ViewField): ViewOperator[] {
  switch (field) {
    case "ENVIRONMENT":
    case "TIME_RANGE":
      return ["IS"];
    case "TEXT_SEARCH":
      return ["CONTAINS"];
    default:
      return ["IS", "IS_NOT"];
  }
}

/** One condition row in the editor. `key` keys the row in the list only. */
export interface ViewFilterRow {
  key: string;
  field: ViewField;
  operator: ViewOperator;
  values: string[];
}

export interface ViewFilterDocument extends Record<string, unknown> {
  filters: { field: FilterField; operator: ViewOperator; values: string[] }[];
  version: number;
  /** The view's default range: one preset. */
  timeRange?: { relative: string };
  /** A metrics view's display options, Render's, kept as written. */
  display?: Record<string, unknown>;
}

let rowCounter = 0;
function rowKey(): string {
  rowCounter += 1;
  return `filter-${rowCounter}`;
}

function isFilterField(value: unknown): value is FilterField {
  return value !== "TIME_RANGE" && VIEW_FIELDS.some((f) => f === value);
}

function isObject(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function stringValues(values: unknown): string[] {
  return Array.isArray(values)
    ? values.filter((v): v is string => typeof v === "string")
    : [];
}

/**
 * A view document's default range, the preset of its top-level
 * `timeRange.relative`, or of a `TIME_RANGE` row in bex's earlier shape.
 * Undefined when it names none.
 */
export function documentTimeRange(
  doc: Record<string, unknown> | undefined,
): string | undefined {
  const range = doc?.timeRange;
  if (isObject(range) && typeof range.relative === "string" && range.relative) {
    return range.relative;
  }
  const filters = doc?.filters;
  if (!Array.isArray(filters)) return undefined;
  for (const raw of filters) {
    if (isObject(raw) && raw.field === "TIME_RANGE" && raw.operator === "IS") {
      return stringValues(raw.values)[0];
    }
  }
  return undefined;
}

/**
 * The rows of a stored document, keeping only what the editor can show: a
 * field of this view's type, one of the field's operators and string values,
 * then the time range as its own row. bex-api validated the document when it
 * stored it, so nothing is normally dropped. A view bex stored before w3/083
 * may still spell Text Search `TEXT` with `IS`, and its range as a
 * `TIME_RANGE` row; bex-api reads both, and so does the editor.
 */
export function rowsFromDocument(
  doc: Record<string, unknown> | undefined,
  type: SavedSearchType,
): ViewFilterRow[] {
  const filters = doc?.filters;
  const allowed = VIEW_FIELDS_BY_TYPE[type];
  const rows: ViewFilterRow[] = [];
  for (const raw of Array.isArray(filters) ? filters : []) {
    if (!isObject(raw)) continue;
    const legacyText = raw.field === "TEXT";
    const field = legacyText ? "TEXT_SEARCH" : raw.field;
    const operator =
      legacyText && raw.operator === "IS" ? "CONTAINS" : raw.operator;
    if (!isFilterField(field) || !allowed.includes(field)) continue;
    if (!viewFieldOperators(field).some((op) => op === operator)) continue;
    rows.push({
      key: rowKey(),
      field,
      operator: operator as ViewOperator,
      values: stringValues(raw.values),
    });
  }
  // Every view type takes a time range.
  const range = documentTimeRange(doc);
  if (range) {
    rows.push({
      key: rowKey(),
      field: "TIME_RANGE",
      operator: "IS",
      values: [range],
    });
  }
  return rows;
}

/**
 * The document a set of rows saves as, in Render's shape: the Time Range row
 * is the top-level `timeRange`, and the stored document's `display` (a
 * metrics view's display options, which the editor does not show) is kept.
 */
export function documentFromRows(
  rows: ViewFilterRow[],
  stored?: Record<string, unknown>,
): ViewFilterDocument {
  const doc: ViewFilterDocument = { filters: [], version: VIEW_FILTER_VERSION };
  for (const { field, operator, values } of rows) {
    if (field === "TIME_RANGE") {
      if (values[0]) doc.timeRange = { relative: values[0] };
    } else {
      doc.filters.push({ field, operator, values });
    }
  }
  if (isObject(stored?.display)) doc.display = stored.display;
  return doc;
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
