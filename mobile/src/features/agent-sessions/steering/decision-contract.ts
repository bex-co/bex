export type DecisionFieldType =
  "string" | "number" | "integer" | "boolean" | "array";

export type DecisionOption = { value: string; label: string };

export type DecisionField = {
  name: string;
  title: string;
  description?: string;
  type: DecisionFieldType;
  required: boolean;
  options: DecisionOption[];
  defaultValue?: unknown;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  minItems?: number;
  maxItems?: number;
};

export type ParsedDecisionContract =
  | { ok: true; fields: DecisionField[] }
  | { ok: false; reason: "invalid" | "sensitive" };

const sensitiveMarker =
  /password|passcode|api[ _-]?key|access[ _-]?token|auth[ _-]?token|private[ _-]?key|credential|secret|one[ -]?time[ -]?code|\botp\b/i;

export function parseDecisionContract(raw: string): ParsedDecisionContract {
  let decoded: unknown;
  try {
    decoded = JSON.parse(raw);
  } catch {
    return { ok: false, reason: "invalid" };
  }
  if (
    !isRecord(decoded) ||
    (decoded.type != null && decoded.type !== "object")
  ) {
    return { ok: false, reason: "invalid" };
  }
  const properties = decoded.properties;
  if (!isRecord(properties)) return { ok: false, reason: "invalid" };
  const entries = Object.entries(properties);
  if (entries.length === 0 || entries.length > 50) {
    return { ok: false, reason: "invalid" };
  }
  const required = new Set(
    Array.isArray(decoded.required)
      ? decoded.required.filter(
          (value): value is string => typeof value === "string",
        )
      : [],
  );
  const fields: DecisionField[] = [];
  for (const [name, value] of entries) {
    if (!name || !isRecord(value) || !isFieldType(value.type)) {
      return { ok: false, reason: "invalid" };
    }
    const title = stringOr(value.title, name);
    const description = stringOr(value.description, "");
    if (sensitiveMarker.test(`${name} ${title} ${description}`)) {
      return { ok: false, reason: "sensitive" };
    }
    const options = parseOptions(
      value.type === "array" && isRecord(value.items) ? value.items : value,
    );
    if (options == null || (value.type === "array" && options.length === 0)) {
      return { ok: false, reason: "invalid" };
    }
    fields.push({
      name,
      title,
      description: description || undefined,
      type: value.type,
      required: required.has(name),
      options,
      defaultValue: value.default,
      minimum: finite(value.minimum),
      maximum: finite(value.maximum),
      minLength: nonnegativeInteger(value.minLength),
      maxLength: nonnegativeInteger(value.maxLength),
      minItems: nonnegativeInteger(value.minItems),
      maxItems: nonnegativeInteger(value.maxItems),
    });
  }
  if ([...required].some((name) => !properties[name])) {
    return { ok: false, reason: "invalid" };
  }
  return { ok: true, fields };
}

export function initialDecisionValues(
  fields: DecisionField[],
): Record<string, unknown> {
  return Object.fromEntries(
    fields
      .filter((field) => field.defaultValue !== undefined)
      .map((field) => [field.name, field.defaultValue]),
  );
}

export function buildDecisionResponse(
  fields: DecisionField[],
  values: Record<string, unknown>,
): { ok: true; value: Record<string, unknown> } | { ok: false } {
  const output: Record<string, unknown> = {};
  for (const field of fields) {
    const raw = values[field.name];
    if (raw == null || raw === "") {
      if (field.required) return { ok: false };
      continue;
    }
    if (field.type === "string") {
      if (typeof raw !== "string") return { ok: false };
      const length = [...raw].length;
      if (
        (field.minLength != null && length < field.minLength) ||
        (field.maxLength != null && length > field.maxLength) ||
        (field.options.length > 0 &&
          !field.options.some((option) => option.value === raw))
      ) {
        return { ok: false };
      }
      output[field.name] = raw;
      continue;
    }
    if (field.type === "number" || field.type === "integer") {
      const number = typeof raw === "number" ? raw : Number(raw);
      if (
        !Number.isFinite(number) ||
        (field.type === "integer" && !Number.isInteger(number)) ||
        (field.minimum != null && number < field.minimum) ||
        (field.maximum != null && number > field.maximum)
      ) {
        return { ok: false };
      }
      output[field.name] = number;
      continue;
    }
    if (field.type === "boolean") {
      if (typeof raw !== "boolean") return { ok: false };
      output[field.name] = raw;
      continue;
    }
    if (!Array.isArray(raw)) return { ok: false };
    const selected = raw.filter(
      (value): value is string => typeof value === "string",
    );
    if (
      selected.length !== raw.length ||
      new Set(selected).size !== selected.length ||
      (field.minItems != null && selected.length < field.minItems) ||
      (field.maxItems != null && selected.length > field.maxItems) ||
      selected.some(
        (value) => !field.options.some((option) => option.value === value),
      )
    ) {
      return { ok: false };
    }
    output[field.name] = selected;
  }
  return { ok: true, value: output };
}

function parseOptions(value: Record<string, unknown>): DecisionOption[] | null {
  if (Array.isArray(value.enum)) {
    const strings = value.enum.filter(
      (item): item is string => typeof item === "string" && item !== "",
    );
    return strings.length === value.enum.length &&
      new Set(strings).size === strings.length
      ? strings.map((item) => ({ value: item, label: item }))
      : null;
  }
  const alternatives = Array.isArray(value.oneOf)
    ? value.oneOf
    : Array.isArray(value.anyOf)
      ? value.anyOf
      : null;
  if (!alternatives) return [];
  const options: DecisionOption[] = [];
  for (const alternative of alternatives) {
    if (
      !isRecord(alternative) ||
      typeof alternative.const !== "string" ||
      !alternative.const ||
      typeof alternative.title !== "string" ||
      !alternative.title
    ) {
      return null;
    }
    options.push({ value: alternative.const, label: alternative.title });
  }
  return new Set(options.map((option) => option.value)).size === options.length
    ? options
    : null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isFieldType(value: unknown): value is DecisionFieldType {
  return ["string", "number", "integer", "boolean", "array"].includes(
    String(value),
  );
}

function stringOr(value: unknown, fallback: string): string {
  return typeof value === "string" ? value : fallback;
}

function finite(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value)
    ? value
    : undefined;
}

function nonnegativeInteger(value: unknown): number | undefined {
  return typeof value === "number" && Number.isInteger(value) && value >= 0
    ? value
    : undefined;
}
