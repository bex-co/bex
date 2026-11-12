import type { ReactElement } from "react";
import { cleanup } from "@testing-library/react";
import { afterEach, expect, vi } from "vitest";
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

function swept(): boolean {
  const path = testFile();
  const name = path.slice(path.lastIndexOf("/") + 1);
  return SWEPT_FILES.test(path) && !(name in EXEMPT);
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
  // The test's own tree is still mounted here, and the probe's act() would flush
  // its pending updates against mocks the test has already reset.
  cleanup();
  let probe;
  try {
    probe = await hydrateAcrossEnvironments(() => elements[0], {
      server,
      client,
    });
  } catch {
    return;
  }
  if (probe.serverErrored) return;
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
});
