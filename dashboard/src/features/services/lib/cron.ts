// Standard 5-field cron helpers shared by the create form and the Settings
// editor. isValidCron accepts exactly what bex-api's validCronSchedule accepts:
// five fields, each parsed the way github.com/robfig/cron/v3 parses it — the
// parser behind both bex-api's check and the Kubernetes CronJob controller.
// Both sides are tested against one vector table
// (lego/backend/internal/apps/testdata/cron-schedule-vectors.json), so the form
// can neither pass a schedule the server refuses (w1/m145: "0 0 * * 7" previewed
// as "Every Sunday", then failed on submit) nor refuse one it accepts.

type FieldSpec = {
  min: number;
  max: number;
  /** lower-cased names → numeric value (e.g. jan→1, sun→0) */
  names?: ReadonlyMap<string, number>;
};

const MONTHS: ReadonlyMap<string, number> = new Map([
  ["jan", 1],
  ["feb", 2],
  ["mar", 3],
  ["apr", 4],
  ["may", 5],
  ["jun", 6],
  ["jul", 7],
  ["aug", 8],
  ["sep", 9],
  ["oct", 10],
  ["nov", 11],
  ["dec", 12],
]);

const WEEKDAYS: ReadonlyMap<string, number> = new Map([
  ["sun", 0],
  ["mon", 1],
  ["tue", 2],
  ["wed", 3],
  ["thu", 4],
  ["fri", 5],
  ["sat", 6],
]);

// Field order: minute, hour, day-of-month, month, day-of-week, with robfig's
// bounds (spec.go). Day-of-week is 0-6: unlike Vixie cron, 7 is not Sunday.
const FIELDS: FieldSpec[] = [
  { min: 0, max: 59 },
  { min: 0, max: 23 },
  { min: 1, max: 31 },
  { min: 1, max: 12, names: MONTHS },
  { min: 0, max: 6, names: WEEKDAYS },
];

// robfig's mustParseInt: Go's strconv.Atoi (an optional "+", decimal digits
// that fit an int64) with negatives refused.
function parseUint(raw: string): number | null {
  const digits = /^\+?0*(\d+)$/.exec(raw)?.[1];
  if (digits === undefined) return null;
  const overflows =
    digits.length > 19 ||
    (digits.length === 19 && digits > "9223372036854775807");
  return overflows ? null : Number(digits);
}

function parseIntOrName(raw: string, spec: FieldSpec): number | null {
  return spec.names?.get(raw.toLowerCase()) ?? parseUint(raw);
}

// validRange ports robfig's getRange for one list item — number, or
// number-number, optionally /step, where "*" or "?" stands for the whole field.
// Its quirks are kept on purpose (the vector table pins each one): anything
// after "*-" is ignored, and "N/step" means "N-max/step".
function validRange(expr: string, spec: FieldSpec): boolean {
  const rangeAndStep = expr.split("/");
  const lowAndHigh = rangeAndStep[0].split("-");
  let start: number | null = spec.min;
  let end: number | null = spec.max;
  if (lowAndHigh[0] !== "*" && lowAndHigh[0] !== "?") {
    if (lowAndHigh.length > 2) return false;
    start = parseIntOrName(lowAndHigh[0], spec);
    end = lowAndHigh.length === 1 ? start : parseIntOrName(lowAndHigh[1], spec);
  }
  if (start === null || end === null || rangeAndStep.length > 2) return false;
  let step = 1;
  if (rangeAndStep.length === 2) {
    const parsed = parseUint(rangeAndStep[1]);
    if (parsed === null) return false;
    step = parsed;
    if (lowAndHigh.length === 1) end = spec.max;
  }
  return start >= spec.min && end <= spec.max && start <= end && step !== 0;
}

/** Returns true if s is a valid standard 5-field cron expression. */
export function isValidCron(s: string): boolean {
  const fields = s.trim().split(/\s+/);
  if (fields.length !== 5) return false;
  // robfig splits a list with strings.FieldsFunc, which drops empty items.
  return fields.every((field, i) =>
    field
      .split(",")
      .filter(Boolean)
      .every((expr) => validRange(expr, FIELDS[i])),
  );
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
// same lookup tables parseIntOrName uses, and a bare "?" to "*" (robfig's
// synonym), so describeCron's phrase branches treat "0 0 * * MON" exactly like
// "0 0 * * 1" and "0 0 ? * *" like "0 0 * * *". Compound terms (ranges, lists,
// steps) pass through untouched — the phrase branches don't describe those.
function canonicalField(field: string, spec: FieldSpec): string {
  if (field === "?") return "*";
  const named = spec.names?.get(field.toLowerCase());
  return named === undefined ? field : String(named);
}

function hhmm(hour: string, minute: string): string {
  return `${hour.padStart(2, "0")}:${minute.padStart(2, "0")}`;
}

// A "*/step" field fires at min, min+step, min+2*step … up to max, then waits
// for the field to wrap — so "every step" is only a truthful description when
// the step divides the field's width. "*/40" minutes fires at :00 and :40,
// alternating 40- and 20-minute gaps; "0 */5" fires at 0,5,10,15,20 with a
// four-hour gap across midnight. Neither has a uniform interval to name.
// A step wider than the field's span leaves only the start value firing, which
// collapses the schedule to the next unit up (robfig still accepts it — "a step
// wider than the range still names its start", per the shared vector table).
function stepShape(
  step: number,
  spec: FieldSpec,
): "uniform" | "start-only" | "uneven" {
  if (step > spec.max - spec.min) return "start-only";
  return (spec.max - spec.min + 1) % step === 0 ? "uniform" : "uneven";
}

// describeCron renders a valid 5-field expression as a short human-readable
// phrase (Render shows one beside the schedule field), best-effort: it names the
// common shapes and returns null for anything unusual or invalid so the caller
// simply shows no preview. Schedules run in UTC.
export function describeCron(s: string): string | null {
  const t = s.trim();
  if (!isValidCron(t)) return null;
  const [minute, hour, dom, month, dow] = t
    .split(/\s+/)
    .map((field, i) => canonicalField(field, FIELDS[i]));
  const allDates = dom === "*" && month === "*" && dow === "*";

  if (minute === "*" && hour === "*" && allDates) return "Every minute";

  const stepMinute = /^\*\/(\d+)$/.exec(minute);
  if (stepMinute && hour === "*" && allDates) {
    switch (stepShape(Number(stepMinute[1]), FIELDS[0])) {
      case "uniform":
        return `Every ${stepMinute[1]} minutes`;
      // Wider than the minute field: only minute 0 ever fires, so it is hourly
      // — the same schedule as "0 * * * *", and described the same way.
      case "start-only":
        return "Every hour";
      default:
        return null;
    }
  }

  const stepHour = /^\*\/(\d+)$/.exec(hour);
  if (/^\d+$/.test(minute) && stepHour && allDates) {
    switch (stepShape(Number(stepHour[1]), FIELDS[1])) {
      case "uniform":
        return `Every ${stepHour[1]} hours`;
      // Wider than the hour field: only hour 0 ever fires, so it is daily.
      case "start-only":
        return `Every day at ${hhmm("0", minute)}`;
      default:
        return null;
    }
  }

  if (/^\d+$/.test(minute) && hour === "*" && allDates) {
    return minute === "0" ? "Every hour" : `Every hour at minute ${minute}`;
  }

  // Single minute + single hour: a daily/weekly/monthly time.
  if (/^\d+$/.test(minute) && /^\d+$/.test(hour)) {
    const at = `at ${hhmm(hour, minute)}`;
    if (allDates) return `Every day ${at}`;
    if (dom === "*" && month === "*" && /^\d+$/.test(dow)) {
      return `Every ${DAY_NAMES[Number(dow)]} ${at}`;
    }
    if (dow === "*" && month === "*" && /^\d+$/.test(dom)) {
      return `On day ${dom} of every month ${at}`;
    }
  }

  return null;
}
