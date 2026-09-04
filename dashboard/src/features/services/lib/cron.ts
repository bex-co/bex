// Standard 5-field cron helpers shared by the create form and the Settings
// editor. isValidCron mirrors the bex-api server check (which parses with the
// same parser the Kubernetes CronJob controller uses): a field-count-only check
// let malformed-but-5-field schedules like "99 99 * * *" through, where the
// apiserver later rejected the CronJob and flipped the service to Failed. Here
// we range-check every field so the form refuses those up front.

type FieldSpec = {
  min: number;
  max: number;
  /** lower-cased names → numeric value (e.g. jan→1, sun→0) */
  names?: Record<string, number>;
};

const MONTHS: Record<string, number> = {
  jan: 1,
  feb: 2,
  mar: 3,
  apr: 4,
  may: 5,
  jun: 6,
  jul: 7,
  aug: 8,
  sep: 9,
  oct: 10,
  nov: 11,
  dec: 12,
};

const WEEKDAYS: Record<string, number> = {
  sun: 0,
  mon: 1,
  tue: 2,
  wed: 3,
  thu: 4,
  fri: 5,
  sat: 6,
};

// Field order: minute, hour, day-of-month, month, day-of-week. Day-of-week
// accepts 0-7 (0 and 7 both Sunday, matching cron), plus SUN-SAT names.
const FIELDS: FieldSpec[] = [
  { min: 0, max: 59 },
  { min: 0, max: 23 },
  { min: 1, max: 31 },
  { min: 1, max: 12, names: MONTHS },
  { min: 0, max: 7, names: WEEKDAYS },
];

function resolveValue(raw: string, spec: FieldSpec): number | null {
  const named = spec.names?.[raw.toLowerCase()];
  if (named !== undefined) return named;
  if (!/^\d+$/.test(raw)) return null;
  const n = Number(raw);
  return n >= spec.min && n <= spec.max ? n : null;
}

// validField validates one cron field: "*", "*/step", or a comma list of terms
// where each term is a value, a "lo-hi" range, or either followed by "/step".
function validField(field: string, spec: FieldSpec): boolean {
  if (field === "") return false;
  return field.split(",").every((term) => {
    if (term === "") return false;
    let body = term;
    const slash = term.indexOf("/");
    if (slash !== -1) {
      const step = term.slice(slash + 1);
      if (!/^\d+$/.test(step) || Number(step) === 0) return false;
      body = term.slice(0, slash);
    }
    if (body === "*") return true;
    const dash = body.indexOf("-");
    if (dash !== -1) {
      const lo = resolveValue(body.slice(0, dash), spec);
      const hi = resolveValue(body.slice(dash + 1), spec);
      return lo !== null && hi !== null && lo <= hi;
    }
    return resolveValue(body, spec) !== null;
  });
}

/** Returns true if s is a valid standard 5-field cron expression. */
export function isValidCron(s: string): boolean {
  const fields = s.trim().split(/\s+/);
  if (fields.length !== 5) return false;
  return fields.every((field, i) => validField(field, FIELDS[i]));
}

const DAY_NAMES = [
  "Sunday",
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
];

// A whole-field named token (MON, JAN) rewritten to its numeric form via the
// same lookup tables resolveValue uses, so describeCron's digit-based phrase
// branches treat "0 0 * * MON" exactly like "0 0 * * 1". Compound terms
// (ranges, lists, steps) pass through untouched — the numeric phrase branches
// don't describe those shapes either.
function canonicalField(field: string, spec: FieldSpec): string {
  const named = spec.names?.[field.toLowerCase()];
  return named === undefined ? field : String(named);
}

function hhmm(hour: string, minute: string): string {
  return `${hour.padStart(2, "0")}:${minute.padStart(2, "0")}`;
}

// describeCron renders a valid 5-field expression as a short human-readable
// phrase (Render shows one beside the schedule field), best-effort: it names the
// common shapes and returns null for anything unusual or invalid so the caller
// simply shows no preview. Schedules run in UTC.
export function describeCron(s: string): string | null {
  const t = s.trim();
  if (!isValidCron(t)) return null;
  const [minute, hour, dom, rawMonth, rawDow] = t.split(/\s+/);
  const month = canonicalField(rawMonth, FIELDS[3]);
  const dow = canonicalField(rawDow, FIELDS[4]);
  const allDates = dom === "*" && month === "*" && dow === "*";

  if (minute === "*" && hour === "*" && allDates) return "Every minute";

  const stepMinute = /^\*\/(\d+)$/.exec(minute);
  if (stepMinute && hour === "*" && allDates)
    return `Every ${stepMinute[1]} minutes`;

  const stepHour = /^\*\/(\d+)$/.exec(hour);
  if (/^\d+$/.test(minute) && stepHour && allDates)
    return `Every ${stepHour[1]} hours`;

  if (/^\d+$/.test(minute) && hour === "*" && allDates) {
    return minute === "0" ? "Every hour" : `Every hour at minute ${minute}`;
  }

  // Single minute + single hour: a daily/weekly/monthly time.
  if (/^\d+$/.test(minute) && /^\d+$/.test(hour)) {
    const at = `at ${hhmm(hour, minute)}`;
    if (allDates) return `Every day ${at}`;
    if (dom === "*" && month === "*" && /^\d+$/.test(dow)) {
      return `Every ${DAY_NAMES[Number(dow) % 7]} ${at}`;
    }
    if (dow === "*" && month === "*" && /^\d+$/.test(dom)) {
      return `On day ${dom} of every month ${at}`;
    }
  }

  return null;
}
