export type EnvironmentFailure =
  | "authorization-denied"
  | "update-restored"
  | "revision-conflict"
  | "revision-invalid"
  | "revision-unavailable"
  | "secrets-unavailable"
  | "source-failed"
  | "projection-failed"
  | "compensation-failed"
  | "timeout-unknown"
  | "not-found"
  | "failed";

type ErrorFacts = {
  codes: Set<string>;
  statuses: Set<number>;
  names: Set<string>;
  messages: Set<string>;
};

export function classifyEnvironmentFailure(error: unknown): EnvironmentFailure {
  const facts = collectFacts(error);
  if (
    facts.statuses.has(401) ||
    facts.statuses.has(403) ||
    matches(facts.codes, /(?:UNAUTHORIZED|FORBIDDEN|AUTHORIZATION)/) ||
    matches(facts.messages, /(?:unauthorized|forbidden|not authorized)/)
  ) {
    return "authorization-denied";
  }
  if (facts.codes.has("ENVIRONMENT_REVISION_CONFLICT")) {
    return "revision-conflict";
  }
  if (facts.codes.has("ENVIRONMENT_REVISION_INVALID")) {
    return "revision-invalid";
  }
  if (facts.codes.has("ENVIRONMENT_UPDATE_RESTORED")) {
    return "update-restored";
  }
  if (facts.codes.has("ENVIRONMENT_RESTORATION_FAILED")) {
    return "compensation-failed";
  }
  if (facts.statuses.has(409) || matches(facts.codes, /(?:CONFLICT|STALE)/)) {
    return "revision-conflict";
  }
  if (
    facts.names.has("TypeError") ||
    facts.statuses.has(408) ||
    [...facts.statuses].some((status) => status >= 500) ||
    facts.names.has("AbortError") ||
    facts.names.has("TimeoutError") ||
    matches(facts.codes, /(?:TIMEOUT|NETWORK|FETCH)/) ||
    matches(
      facts.messages,
      /(?:timeout|timed out|deadline exceeded|network request failed|failed to fetch)/,
    )
  ) {
    return "timeout-unknown";
  }
  if (
    matches(facts.codes, /(?:SECRETS?|OPENBAO).*UNAVAILABLE/) ||
    matches(facts.messages, /(?:secret store|openbao).*unavailable/)
  ) {
    return "secrets-unavailable";
  }
  if (
    matches(facts.codes, /COMPENSAT/) ||
    matches(facts.messages, /compensat/)
  ) {
    return "compensation-failed";
  }
  if (
    matches(facts.codes, /PROJECTION/) ||
    matches(facts.messages, /project(?:ion|ing)/)
  ) {
    return "projection-failed";
  }
  if (
    matches(facts.codes, /(?:SOURCE|OPENBAO|VAULT)/) ||
    matches(facts.messages, /(?:source|openbao|vault)/)
  ) {
    return "source-failed";
  }
  if (
    facts.statuses.has(404) ||
    matches(facts.codes, /NOT_FOUND/) ||
    matches(facts.messages, /not found/)
  ) {
    return "not-found";
  }
  return "failed";
}

function collectFacts(error: unknown): ErrorFacts {
  const facts: ErrorFacts = {
    codes: new Set(),
    statuses: new Set(),
    names: new Set(),
    messages: new Set(),
  };
  const seen = new Set<unknown>();
  const visit = (value: unknown) => {
    if (!value || typeof value !== "object" || seen.has(value)) return;
    seen.add(value);
    const record = value as Record<string, unknown>;
    if (typeof record.code === "string")
      facts.codes.add(record.code.toUpperCase());
    if (typeof record.name === "string") facts.names.add(record.name);
    if (typeof record.message === "string")
      facts.messages.add(record.message.toLowerCase());
    for (const key of ["status", "statusCode"])
      if (typeof record[key] === "number") facts.statuses.add(record[key]);
    for (const key of ["extensions", "cause"])
      if (record[key]) visit(record[key]);
    for (const key of ["errors", "graphQLErrors"])
      if (Array.isArray(record[key]))
        for (const nested of record[key]) visit(nested);
  };
  visit(error);
  return facts;
}

function matches(values: Set<string>, pattern: RegExp): boolean {
  for (const value of values) if (pattern.test(value)) return true;
  return false;
}
