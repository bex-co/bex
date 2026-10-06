import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeployLogPanel } from "../deploy-log-panel";
import { setupVirtualGeometry } from "@/test/virtual-geometry";

// The real useDeployLogs merge (toLogLines + dedupe) under the real panel
// filter. Only Apollo's useQuery is stubbed, per typed leg: the same pre-deploy
// record comes back from both the durable `build` leg and the `predeploy` leg.
const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useApolloClient: () => ({ query: vi.fn() }),
}));

setupVirtualGeometry();

const POD = "srv-db0ruak48ccs739jikeg-socdldpjj65dr8tqj7v3";
const MARKER = "QA_R83_PREDEPLOY_FAILURE";

const entry = (
  timestamp: string,
  message: string,
  type: string,
  instance = POD,
) => ({
  __typename: "LogEntry",
  timestamp,
  message,
  type,
  instance,
  level: null,
  method: null,
  statusCode: null,
});

function stubLegs(legs: Record<string, unknown[]>) {
  const data = new Map(
    Object.entries(legs).map(([type, logs]) => [
      type,
      {
        logs: {
          __typename: "LogList",
          hasMore: false,
          nextStartTime: "2026-10-04T02:54:00Z",
          nextEndTime: "2026-10-04T02:56:00Z",
          logs,
        },
      },
    ]),
  );
  mockUseQuery.mockImplementation(
    (_doc: unknown, opts?: { variables?: { type?: string } }) => ({
      data: data.get(opts?.variables?.type ?? ""),
      loading: false,
      error: undefined,
    }),
  );
}

const twin = (type: string) =>
  entry("2026-10-04T02:54:41.610280087Z", MARKER, type);
const buildOnly = entry("2026-10-04T02:54:30Z", "==> Building", "build");
const appOnly = entry("2026-10-04T02:54:50Z", "GET /qa-r83", "app");

async function choose(name: string) {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Log type" }));
  await user.click(screen.getByRole("menuitemradio", { name }));
}

function renderPanel() {
  render(
    <DeployLogPanel
      resource="srv-db0ruak48ccs739jikeg"
      startTime="2026-10-04T02:54:00Z"
      endTime="2026-10-04T02:56:00Z"
      hasPreDeploy
      followBuild={false}
    />,
  );
}

const SEARCH_PLACEHOLDER = /search/i;

const count = (text: string) => screen.queryAllByText(text).length;

beforeEach(() => mockUseQuery.mockReset());

describe("DeployLogPanel source buckets over the real merge", () => {
  it.each([
    [
      "build leg first",
      { build: [buildOnly, twin("build")], predeploy: [twin("predeploy")] },
    ],
    [
      "predeploy leg only so far, durable twin later",
      { build: [buildOnly], predeploy: [twin("predeploy")] },
    ],
  ])(
    "keeps a pre-deploy line once in All, Build and Application (%s)",
    async (_case, legs) => {
      stubLegs({ ...legs, app: [appOnly] });
      renderPanel();

      expect(count(MARKER)).toBe(1); // All: one copy, never two

      await choose("Application logs");
      expect(count(MARKER)).toBe(1);
      expect(count("GET /qa-r83")).toBe(1);
      expect(count("==> Building")).toBe(0);

      await choose("Build logs");
      // Only the durable twin makes it build output.
      expect(count(MARKER)).toBe(legs.build.length > 1 ? 1 : 0);
      expect(count("==> Building")).toBe(1);
      expect(count("GET /qa-r83")).toBe(0);
    },
  );

  it("finds the pre-deploy line by search inside Application logs", async () => {
    stubLegs({
      build: [buildOnly, twin("build")],
      predeploy: [twin("predeploy")],
      app: [appOnly],
    });
    renderPanel();
    await choose("Application logs");
    const user = userEvent.setup();
    await user.type(screen.getByPlaceholderText(SEARCH_PLACEHOLDER), MARKER);
    // Search is debounced: the non-matching app line leaves once it applies.
    await waitFor(() => expect(count("GET /qa-r83")).toBe(0));
    expect(count(MARKER)).toBe(1);
  });
});
