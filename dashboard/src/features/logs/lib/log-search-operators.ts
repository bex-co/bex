import type { LogFilters, StructuredFilterKey } from "@/features/logs/types";

// Render's log search field operators (render.com/docs/logging): a
// `name:value` term filters on that field instead of matching text, with the
// value's literal, `*` wildcard or `/RE2/` semantics the server already
// implements (w8/176). Typed into the search box they used to be sent as
// literal text and matched nothing (w4/538).
const SEARCH_OPERATORS: Record<string, StructuredFilterKey> = {
  level: "level",
  instance: "instance",
  method: "method",
  status_code: "statusCode",
  host: "host",
  path: "path",
};

const OPERATOR_TERM = /^([A-Za-z_]+):(\S+)$/;

/**
 * Splits a search string into Render's field operators and the remaining
 * free text. An unknown `name:` stays text, and so does an operator with no
 * value; a repeated operator keeps its last value.
 */
export function parseSearchOperators(text: string): {
  text: string;
  filters: Partial<Pick<LogFilters, StructuredFilterKey>>;
} {
  const filters: Partial<Pick<LogFilters, StructuredFilterKey>> = {};
  const rest: string[] = [];
  for (const term of text.trim().split(/\s+/)) {
    const match = OPERATOR_TERM.exec(term);
    const key = match && SEARCH_OPERATORS[match[1].toLowerCase()];
    if (match && key) filters[key] = match[2];
    else if (term) rest.push(term);
  }
  return { text: rest.join(" "), filters };
}
