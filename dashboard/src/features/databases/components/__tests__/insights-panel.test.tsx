import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

type SpecRow = { name: string; value: string };

const state: {
  processes: ProcessRow[];
  topQueries: TopQueryRow[];
  parameterSpec: SpecRow[];
} = {
  processes: [],
  topQueries: [],
  parameterSpec: [],
};
const saveParameters = vi.fn();

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
    parameterSpec: state.parameterSpec,
    parameterSpecLoading: false,
    parameterSpecError: undefined,
    saving: false,
    saveParameters,
    refetchAll: vi.fn(),
  }),
}));

beforeEach(() => {
  state.processes = [];
  state.topQueries = [];
  state.parameterSpec = [];
  saveParameters.mockReset();
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

// At 390px the bare Top queries table kept its 320px query cell while Calls/
// Total/Mean collapsed to character-wide strips, and Mean ran past the card
// edge with no way to reach it. The table now pans inside its own region and
// keeps every timing on one line; layout itself is verified in a browser.
describe("InsightsPanel Top queries on a narrow screen", () => {
  it("pans locally and keeps timing headings and values on one line", () => {
    state.topQueries = [
      topQuery({ query: "SELECT pg_sleep(1.2)", totalTimeMs: 1197.785851 }),
    ];

    render(<InsightsPanel id="dpg-1" />);

    const table = screen.getByText("SELECT pg_sleep(1.2)").closest("table")!;
    expect(table.parentElement).toHaveAttribute(
      "data-slot",
      "top-queries-scroll",
    );
    expect(table.parentElement).toHaveClass("overflow-x-auto");
    expect(table).toHaveClass("min-w-[36rem]");

    const headings = [...table.querySelectorAll("th")].slice(1);
    const values = [...table.querySelectorAll("tbody td")].slice(1);
    expect(headings).toHaveLength(3);
    for (const cell of [...headings, ...values]) {
      expect(cell).toHaveClass("whitespace-nowrap");
    }
    expect(values[1]).toHaveTextContent("1197.8ms");
  });
});

// A poll or another tab's save delivers a new saved set while this tab holds
// an unsaved draft. The panel used to key the editor by the saved values, so
// the new set remounted it and silently dropped the draft (w4/253).
describe("InsightsPanel parameter overrides refresh", () => {
  const value = () => screen.getByLabelText("Parameter 1 value");
  const saveButton = () =>
    screen.getByRole("button", { name: "Save overrides" });
  const discard = () =>
    screen.queryByRole("button", { name: "Discard changes" });

  function deliver(
    rerender: (ui: React.ReactElement) => void,
    spec: SpecRow[],
    id = "dpg-1",
  ) {
    state.parameterSpec = spec;
    rerender(<InsightsPanel id={id} />);
  }

  it("keeps a dirty draft when another tab saves, and Discard picks the latest saved value", async () => {
    const user = userEvent.setup();
    state.parameterSpec = [{ name: "work_mem", value: "8MB" }];
    const { rerender } = render(<InsightsPanel id="dpg-1" />);

    await user.clear(value());
    await user.type(value(), "16MB");
    deliver(rerender, [{ name: "work_mem", value: "12MB" }]);

    expect(value()).toHaveValue("16MB");
    expect(saveButton()).toBeEnabled();
    expect(discard()).toBeInTheDocument();
    expect(saveParameters).not.toHaveBeenCalled();

    await user.click(discard()!);
    expect(value()).toHaveValue("12MB");
    expect(saveButton()).toBeDisabled();
  });

  it("refreshes an untouched editor to the latest saved set, including added and removed rows", () => {
    state.parameterSpec = [{ name: "work_mem", value: "8MB" }];
    const { rerender } = render(<InsightsPanel id="dpg-1" />);

    deliver(rerender, [
      { name: "random_page_cost", value: "1.25" },
      { name: "work_mem", value: "12MB" },
    ]);
    expect(screen.getByLabelText("Parameter 1 name")).toHaveValue(
      "random_page_cost",
    );
    expect(screen.getByLabelText("Parameter 2 value")).toHaveValue("12MB");
    expect(saveButton()).toBeDisabled();

    deliver(rerender, []);
    expect(screen.queryByLabelText("Parameter 1 name")).not.toBeInTheDocument();
  });

  it("adopts its own successful save and then follows later saved sets", async () => {
    const user = userEvent.setup();
    saveParameters.mockResolvedValue({ ok: true });
    state.parameterSpec = [{ name: "work_mem", value: "8MB" }];
    const { rerender } = render(<InsightsPanel id="dpg-1" />);

    await user.clear(value());
    await user.type(value(), "16MB");
    await user.click(saveButton());
    expect(saveParameters).toHaveBeenCalledWith([
      { name: "work_mem", value: "16MB" },
    ]);
    expect(saveButton()).toBeDisabled();
    expect(discard()).not.toBeInTheDocument();

    deliver(rerender, [{ name: "work_mem", value: "16MB" }]);
    deliver(rerender, [{ name: "work_mem", value: "24MB" }]);
    expect(value()).toHaveValue("24MB");
  });

  it("keeps a failed save's draft through a refresh", async () => {
    const user = userEvent.setup();
    saveParameters.mockResolvedValue({ ok: false, error: "bad value" });
    state.parameterSpec = [{ name: "work_mem", value: "8MB" }];
    const { rerender } = render(<InsightsPanel id="dpg-1" />);

    await user.clear(value());
    await user.type(value(), "nope");
    await user.click(saveButton());
    expect(screen.getByRole("alert")).toHaveTextContent("bad value");

    deliver(rerender, [{ name: "work_mem", value: "12MB" }]);
    expect(value()).toHaveValue("nope");
  });

  it("resets the editor for a different database", async () => {
    const user = userEvent.setup();
    state.parameterSpec = [{ name: "work_mem", value: "8MB" }];
    const { rerender } = render(<InsightsPanel id="dpg-1" />);

    await user.clear(value());
    await user.type(value(), "16MB");
    deliver(rerender, [{ name: "work_mem", value: "8MB" }], "dpg-2");

    expect(value()).toHaveValue("8MB");
    expect(discard()).not.toBeInTheDocument();
  });
});
