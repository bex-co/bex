import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  SERVICE_EVENT_TYPES,
  serviceEventHasExplicitLabel,
} from "@/features/events/service-event-catalog";

// The cross-boundary drift guard (w6/m122 t003). Three guards already cover the
// service event feed — scripts/events-verify.sh, TestEventSurfaceParity, and
// service-event-filter.test.tsx — and none crosses the Go/TypeScript boundary,
// which is how five emitted types (custom_domain_verified, disk_created,
// disk_updated, disk_deleted, disk_restored) drifted out of the dashboard
// catalog unnoticed after w7/m66 built it.
//
// This enumerates the vocabulary FROM THE GO SOURCE. A restated TypeScript list
// would just move the drift one file over, so nothing here names an event type:
// every string below is a Go identifier or a structural anchor.
//
// Both vocabulary packages are covered. internal/eventvocab is the split half —
// plan_changed lives there, not in internal/events — and a guard reading only
// service.go would give false confidence.

const REPO_ROOT = `${process.cwd()}/..`;
const EVENTS_GO = `${REPO_ROOT}/lego/backend/internal/events/service.go`;
const EVENTVOCAB_GO = `${REPO_ROOT}/lego/backend/internal/eventvocab/datastore.go`;

/** `TypeFoo = "foo"` → {TypeFoo: "foo"} for one Go file. */
function stringConstants(source: string): Map<string, string> {
  const out = new Map<string, string>();
  for (const [, name, value] of source.matchAll(
    /^\s*(Type[A-Za-z0-9_]*)\s*=\s*"([^"]*)"/gm,
  )) {
    out.set(name, value);
  }
  return out;
}

/** `TypeFoo = eventvocab.TypeBar` → {TypeFoo: "bar"}, resolved through vocab. */
function aliasedConstants(
  source: string,
  vocab: Map<string, string>,
): Map<string, string> {
  const out = new Map<string, string>();
  for (const [, name, target] of source.matchAll(
    /^\s*(Type[A-Za-z0-9_]*)\s*=\s*eventvocab\.(Type[A-Za-z0-9_]*)/gm,
  )) {
    const value = vocab.get(target);
    if (value !== undefined) out.set(name, value);
  }
  return out;
}

/** The identifiers a `name = map[...]{...}` / `name = []string{...}` block references. */
function blockValues(source: string, anchor: RegExp): string[] {
  const start = source.search(anchor);
  if (start === -1) return [];
  const open = source.indexOf("{", start);
  let depth = 0;
  let end = open;
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === "{") depth += 1;
    else if (source[i] === "}") {
      depth -= 1;
      if (depth === 0) {
        end = i;
        break;
      }
    }
  }
  return [...source.slice(open, end).matchAll(/\b(Type[A-Za-z0-9_]*)\b/g)].map(
    ([, name]) => name,
  );
}

const eventsSource = readFileSync(EVENTS_GO, "utf8");
const vocabSource = readFileSync(EVENTVOCAB_GO, "utf8");

const vocabConstants = stringConstants(vocabSource);
const eventsConstants = new Map([
  ...stringConstants(eventsSource),
  ...aliasedConstants(eventsSource, vocabConstants),
]);

// The datastore-only half, derived rather than hand-listed: every type
// eventvocab maps for datastore audit rows that internal/events does NOT also
// route into a service feed. service.go says so itself where it declares
// indexedAuditEventTypes — those rows "have no service-scoped list home" and are
// deliberately kept out of eventTypes. plan_changed is the one member of
// eventvocab that IS a service event (apps.SetPlan), and it survives this
// subtraction because eventTypes references it.
const serviceVerbTypes = new Set(
  blockValues(eventsSource, /var\s+eventTypes\s*=\s*map\[string\]string/).map(
    (name) => eventsConstants.get(name) ?? name,
  ),
);
const datastoreOnly = new Set(
  [...blockValues(vocabSource, /func\s+DatastoreAuditTypes\(\)/)]
    .map((name) => vocabConstants.get(name) ?? name)
    .filter((type) => !serviceVerbTypes.has(type)),
);

/** Every event type GET /services/{id}/events can return. */
const backendVocabulary = [
  ...new Set(
    [...eventsConstants.values()].filter((type) => !datastoreOnly.has(type)),
  ),
].sort();

describe("backend service-event vocabulary", () => {
  it("parses the Go source rather than silently reading nothing", () => {
    // Without this, a refactor that moves or reshapes the constants turns the
    // subset assertions below into vacuous truths that pass forever.
    expect(
      backendVocabulary.length,
      `Parsed no event types out of ${EVENTS_GO}. The guard's regexes no longer match the Go source — fix the parser, do not delete the test.`,
    ).toBeGreaterThan(40);
    expect(
      datastoreOnly.size,
      `Parsed no datastore-only types out of ${EVENTVOCAB_GO}. Without them the vocabulary wrongly includes Postgres rows that a service feed never returns.`,
    ).toBeGreaterThan(0);
    expect(serviceVerbTypes.size).toBeGreaterThan(0);
  });

  it("is a subset of the dashboard catalog", () => {
    const catalogued = new Set(SERVICE_EVENT_TYPES);
    const missing = backendVocabulary.filter((type) => !catalogued.has(type));

    // Subset, not equality: the dashboard may legitimately still know a type the
    // backend has stopped emitting, so REMOVING a backend type must not fail the
    // build. Only an uncatalogued emitted type does.
    expect(
      missing,
      `These event types are emitted by lego/backend but absent from SERVICE_EVENT_GROUPS. The Events tab renders them under the generic label (it is fail-open since w6/m122), but they have no group, no icon and no name. Add each to service-event-catalog.ts, event-icon.tsx, and the en + zh locales.`,
    ).toEqual([]);
  });

  it("gives every emitted type a label of its own", () => {
    const unlabelled = backendVocabulary.filter(
      (type) => !serviceEventHasExplicitLabel(type),
    );

    expect(
      unlabelled,
      `These event types have no LABEL_KEYS entry, so they render as the generic "Service settings changed". Map each to an i18n key in service-event-catalog.ts (mapping one deliberately to services.eventsTypeServiceChanged is fine — it just has to be deliberate).`,
    ).toEqual([]);
  });
});
