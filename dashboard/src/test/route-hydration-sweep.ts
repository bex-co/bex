import type { ReactElement } from "react";
import { cleanup } from "@testing-library/react";
import { afterEach, expect, vi } from "vitest";
import { a11yViolations } from "./axe";
import {
  BROWSER_ENVIRONMENT,
  CHINESE_BROWSER_ENVIRONMENT,
  SERVER_ENVIRONMENT,
  hydrateAcrossEnvironments,
  type HydrationEnvironment,
} from "./hydration";

// w21/m337: the first tree each page-level test renders is also server-rendered
// in one environment and hydrated in another, after the test's assertions have
// passed, so the test's own fixtures are the page's data and its mocks cost the
// test nothing. A page that renders different text on the two sides (a time
// zone, a clock, a language) fails the test that rendered it, with React's own
// description of the two trees.
//
// Swept: every route test, every `*page*.test.tsx`, and the forms that shipped
// a mismatch. The wrapper below only records the element; the probe runs in
// afterEach.

const SWEPT_FILES =
  /(\/src\/routes\/__tests__\/|page[^/]*\.test\.tsx$|automation-form\.test\.tsx$)/;

// A fixture the probe cannot render soundly, with the reason: it holds the page
// in a state no server renders, so there is no mismatch to find or fix.
const EXEMPT: Record<string, string> = {
  "services.$serviceId.scaling.test.tsx":
    "it holds the route's loader pending on purpose; a server render waits for loaders, so that state is never rendered",
};

// A fixture the accessibility pass cannot judge, with the reason. Empty until
// the first sweep finds one (w21/m395 t002).
const A11Y_EXEMPT = new Map<string, string>();

// The two sides differ in zone and clock; the second also in language (the same
// language on both sides, as in the app, where the server reads the viewer's
// cookie). One pair per test, by name: a probe mounts the tree again, and a
// fixture with state (a feed, a counter) answers its second mount differently.
const PAIRS: Array<{
  server: HydrationEnvironment;
  client: HydrationEnvironment;
}> = [
  { server: SERVER_ENVIRONMENT, client: BROWSER_ENVIRONMENT },
  {
    server: { ...SERVER_ENVIRONMENT, language: "zh" },
    client: CHINESE_BROWSER_ENVIRONMENT,
  },
];

const rendered: ReactElement[] = [];

function testFile(): string {
  return String(expect.getState().testPath);
}

function testFileName(): string {
  const path = testFile();
  return path.slice(path.lastIndexOf("/") + 1);
}

function swept(): boolean {
  return SWEPT_FILES.test(testFile()) && !(testFileName() in EXEMPT);
}

vi.mock("@testing-library/react", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@testing-library/react")>();
  return {
    ...actual,
    render: ((ui: ReactElement, ...rest: unknown[]) => {
      if (swept()) rendered.push(ui);
      return (actual.render as (...args: unknown[]) => unknown)(ui, ...rest);
    }) as typeof actual.render,
  };
});

function pairFor(testName: string) {
  let hash = 0;
  for (const char of testName) hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  return PAIRS[hash % PAIRS.length];
}

// React's message opens with a generic paragraph; the tree diff that names the
// differing element and text follows the link.
function describeMismatch(error: unknown): string {
  const message = String((error as Error)?.message ?? error);
  const link = "https://react.dev/link/hydration-mismatch";
  const at = message.indexOf(link);
  return (at < 0 ? message : message.slice(at + link.length))
    .trim()
    .slice(0, 1500);
}

// The probe runs after the test's own afterEach hooks, so a fixture that resets
// its mocks there (a hook mock that returns undefined again) cannot render the
// page a second time. That is the fixture's end state, not a page that
// mismatches, so a render that errors on either side is skipped, not failed.
// What fails is React reporting that the server markup and the client disagree.
const SERVER_RENDER_ERRORED =
  "Switched to client rendering because the server rendering errored";

afterEach(async () => {
  const elements = rendered.splice(0);
  if (!swept() || elements.length === 0) return;
  // Only the first render of a test: later ones are the same page moved to
  // another state, and the test's mocks have usually been reset by now.
  const { server, client } = pairFor(expect.getState().currentTestName ?? "");
  const other = PAIRS.find((pair) => pair.client !== client)!;
  // The test's own tree is still mounted here, and the probe's act() would flush
  // its pending updates against mocks the test has already reset.
  cleanup();
  // The accessibility pass reads the hydrated page in both languages (w21/m395
  // t002): the probe's own pair, then the other pair, whose hydration result is
  // not judged (a fixture with state answers a second mount differently). The
  // markup is copied while the tree is mounted (the body, so a dialog in a
  // portal is included) and checked after it is gone: axe yields to the event
  // loop, and doing that under a mounted Radix modal left its `pointer-events:
  // none` on the page for the next test.
  const snapshots: Array<{ language: string | undefined; html: string }> = [];
  const snapshot = (language: string | undefined) => () => {
    snapshots.push({ language, html: document.body.innerHTML });
  };
  let probe;
  try {
    probe = await hydrateAcrossEnvironments(() => elements[0], {
      server,
      client,
      inspect: A11Y_EXEMPT.has(testFileName())
        ? undefined
        : snapshot(client.language),
    });
  } catch {
    return;
  }
  if (probe.serverErrored) return;
  if (!A11Y_EXEMPT.has(testFileName())) {
    try {
      await hydrateAcrossEnvironments(() => elements[0], {
        server: other.server,
        client: other.client,
        inspect: snapshot(other.client.language),
      });
    } catch {
      // The fixture cannot mount a second time; the first language was judged.
    }
  }
  const mismatches = probe.recovered.filter(
    (error) =>
      !String((error as Error)?.message ?? error).includes(
        SERVER_RENDER_ERRORED,
      ),
  );
  if (mismatches.length > 0) {
    throw new Error(
      `hydration mismatch (server ${server.timeZone}/${server.language}, browser ${client.timeZone}/${client.language}):\n` +
        mismatches.map(describeMismatch).join("\n---\n"),
    );
  }
  const accessibility: string[] = [];
  for (const { language, html } of snapshots) {
    const host = document.createElement("div");
    host.innerHTML = html;
    document.body.appendChild(host);
    try {
      for (const found of await a11yViolations(host)) {
        accessibility.push(`[${language}] ${found}`);
      }
    } finally {
      host.remove();
    }
  }
  if (accessibility.length > 0) {
    throw new Error(
      `accessibility violations in the rendered page:\n${[...new Set(accessibility)].join("\n")}`,
    );
  }
});
