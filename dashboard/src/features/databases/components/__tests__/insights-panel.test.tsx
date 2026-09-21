import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { InsightsPanel } from "@/features/databases/components/insights-panel";

// The literal PostgreSQL substitutes for a query text the reading role may not
// see. On 2026-09-19 it rendered 34 times on a live database's Insights section
// with no explanation (w4/m115) — it must never appear in the DOM again.
const MASK = "<insufficient privilege>";

type ProcessRow = {
  pid: number;
  userName: string;
  applicationName: string;
  state: string;
  query: string;
  masked?: boolean | null;
  waitEventType: string;
  waitEvent: string;
  durationSeconds: number;
};

type TopQueryRow = {
  query: string;
  masked?: boolean | null;
  calls: number;
  totalTimeMs: number;
  meanTimeMs: number;
  rows: number;
  sharedHitBlks: number;
  sharedReadBlks: number;
};

function process(over: Partial<ProcessRow>): ProcessRow {
  return {
    pid: 1,
    userName: "d_user",
    applicationName: "psql",
    state: "active",
    query: "SELECT 1",
    waitEventType: "",
    waitEvent: "",
    durationSeconds: 2,
    ...over,
  };
}

function topQuery(over: Partial<TopQueryRow>): TopQueryRow {
  return {
    query: "SELECT 1",
    calls: 3,
    totalTimeMs: 12.5,
    meanTimeMs: 4.1,
    rows: 3,
    sharedHitBlks: 0,
    sharedReadBlks: 0,
    ...over,
  };
}

const state: { processes: ProcessRow[]; topQueries: TopQueryRow[] } = {
  processes: [],
  topQueries: [],
};

vi.mock("@/features/databases/hooks/use-database-insights", () => ({
  useDatabaseInsights: () => ({
    processes: state.processes,
    processesLoading: false,
    processesError: undefined,
    topQueries: state.topQueries,
    topQueriesLoading: false,
    topQueriesError: undefined,
    sizes: null,
    sizesLoading: false,
    sizesError: undefined,
    tableScans: [],
    tableScansLoading: false,
    tableScansError: undefined,
    parameterOverrides: [],
    parameterOverridesLoading: false,
    parameterOverridesError: undefined,
    parameterSpec: [],
    parameterSpecLoading: false,
    parameterSpecError: undefined,
    saving: false,
    saveParameters: vi.fn(),
    refetchAll: vi.fn(),
  }),
}));

beforeEach(() => {
  state.processes = [];
  state.topQueries = [];
});

describe("InsightsPanel masked rows", () => {
  it("explains a fully masked section instead of printing the mask literal", () => {
    state.processes = [
      process({ pid: 9, query: "", masked: true }),
      process({ pid: 10, query: "", masked: true }),
    ];
    state.topQueries = [topQuery({ query: "", masked: true })];

    const { container } = render(<InsightsPanel id="dpg-1" />);

    expect(container.textContent).not.toContain(MASK);
    // One explanation per affected sub-section: processes and top queries.
    expect(screen.getAllByText(/hides their SQL/i)).toHaveLength(2);
    expect(screen.getAllByText("Hidden").length).toBe(3);
  });

  it("renders real rows beside the explanation when masking is partial", () => {
    state.processes = [
      process({ pid: 9, query: "", masked: true }),
      process({ pid: 10, query: "SELECT * FROM orders" }),
    ];
    state.topQueries = [topQuery({ query: "SELECT count(*) FROM orders" })];

    const { container } = render(<InsightsPanel id="dpg-1" />);

    expect(container.textContent).not.toContain(MASK);
    expect(screen.getByText("SELECT * FROM orders")).toBeInTheDocument();
    expect(screen.getByText("SELECT count(*) FROM orders")).toBeInTheDocument();
    // Only the processes section has a hidden row, so only it explains.
    expect(screen.getAllByText(/hides their SQL/i)).toHaveLength(1);
    expect(screen.getAllByText("Hidden")).toHaveLength(1);
  });

  it("still suppresses the literal when an older API sends it as query text", () => {
    // Pre-w4/m115 bex-api has no `masked` field and passes PostgreSQL's
    // placeholder through as if it were SQL. The dashboard must not render it.
    state.processes = [process({ pid: 9, query: MASK })];
    state.topQueries = [topQuery({ query: MASK })];

    const { container } = render(<InsightsPanel id="dpg-1" />);

    expect(container.textContent).not.toContain(MASK);
    expect(screen.getAllByText("Hidden")).toHaveLength(2);
  });

  it("says nothing about masking when every row is visible", () => {
    state.processes = [process({ pid: 9 })];
    state.topQueries = [topQuery({})];

    render(<InsightsPanel id="dpg-1" />);

    expect(screen.queryByText(/hides their SQL/i)).not.toBeInTheDocument();
    expect(screen.queryByText("Hidden")).not.toBeInTheDocument();
  });
});
