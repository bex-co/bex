import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { InsightsPanel } from "@/features/databases/components/insights-panel";
import i18n from "@/i18n/init";

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

type SpecRow = {
  name: string;
  value: string;
  observationStatus?: string;
  observedSetting?: string | null;
  observedUnit?: string | null;
};
type SizeRow = { schema: string; name: string; sizePretty: string };
type ScanRow = {
  schema: string;
  name: string;
  seqScans: number;
  indexScans: number;
  liveRows: number;
};

const state: {
  processes: ProcessRow[];
  topQueries: TopQueryRow[];
  topQueriesError: Error | undefined;
  parameterSpec: SpecRow[];
  sizes: { database: null; tables: SizeRow[] } | null;
  tableScans: ScanRow[];
} = {
  processes: [],
  topQueries: [],
  topQueriesError: undefined,
  parameterSpec: [],
  sizes: null,
  tableScans: [],
};

vi.mock("@/features/databases/hooks/use-database-insights", () => ({
  useDatabaseInsights: () => ({
    processes: state.processes,
    processesLoading: false,
    processesError: undefined,
    topQueries: state.topQueries,
    topQueriesLoading: false,
    topQueriesError: state.topQueriesError,
    sizes: state.sizes,
    sizesLoading: false,
    sizesError: undefined,
    tableScans: state.tableScans,
    tableScansLoading: false,
    tableScansError: undefined,
    parameterOverrides: [],
    parameterOverridesLoading: false,
    parameterOverridesError: undefined,
    parameterSpec: state.parameterSpec,
    refetchAll: vi.fn(),
  }),
}));

beforeEach(() => {
  state.processes = [];
  state.topQueries = [];
  state.topQueriesError = undefined;
  state.parameterSpec = [];
  state.sizes = null;
  state.tableScans = [];
});

afterEach(async () => {
  await i18n.changeLanguage("en");
});

// bex-api answers an empty list only when pg_stat_statements answered and holds
// no row of the owner's: a missing extension or an unreachable database is an
// error, which lands on the unavailable line. After w21/058 stopped the panel's
// own envelope statements from filling the list, a quiet database reaches this
// empty state, and it told the owner their cluster was misconfigured (w21/062).
describe("InsightsPanel Top queries empty state", () => {
  function topQueriesSection() {
    const heading = screen.getByRole("heading", {
      name: /top queries|热门查询/i,
    });
    return within(heading.closest("div.space-y-2") as HTMLElement);
  }

  it("says nothing has run, without blaming pg_stat_statements", () => {
    render(<InsightsPanel id="dpg-1" />);

    const section = topQueriesSection();
    expect(section.getByText(/No queries yet/)).toBeInTheDocument();
    expect(
      section.queryByText(/pg_stat_statements|enabled|misconfigured/i),
    ).toBeNull();
  });

  it("drops the empty text once a statement is listed", () => {
    state.topQueries = [topQuery({ query: "SELECT 1 AS qa_one" })];

    render(<InsightsPanel id="dpg-1" />);

    const section = topQueriesSection();
    expect(section.getByText("SELECT 1 AS qa_one")).toBeInTheDocument();
    expect(section.queryByText(/No queries yet/)).toBeNull();
  });

  it("still shows the unavailable line when the read failed", () => {
    state.topQueriesError = new Error("top query statistics could not be read");

    render(<InsightsPanel id="dpg-1" />);

    const section = topQueriesSection();
    expect(section.getByText(/^Unavailable/)).toBeInTheDocument();
    expect(section.queryByText(/No queries yet/)).toBeNull();
  });

  it("reads the same in Chinese", async () => {
    await i18n.changeLanguage("zh");

    render(<InsightsPanel id="dpg-1" />);

    const section = topQueriesSection();
    expect(section.getByText(/暂无查询。/)).toBeInTheDocument();
    expect(section.queryByText(/pg_stat_statements|未启用/)).toBeNull();
  });
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

// parameterOverrides was retired to match Render (w2/066): the editor is gone,
// and a database that still declares parameters shows them read-only.
describe("InsightsPanel declared parameters", () => {
  it("hides the section for a database that declares nothing", () => {
    render(<InsightsPanel id="dpg-1" />);
    expect(screen.queryByText("Parameter overrides")).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("shows declared parameters read-only with their observations", () => {
    state.parameterSpec = [
      {
        name: "max_connections",
        value: "200",
        observationStatus: "not_observed",
      },
      {
        name: "work_mem",
        value: "8MB",
        observationStatus: "observed",
        observedSetting: "8192",
        observedUnit: "kB",
      },
    ];
    render(<InsightsPanel id="dpg-1" />);

    expect(screen.getByText("Parameter overrides")).toBeInTheDocument();
    expect(screen.getByText(/ALTER DATABASE … SET/)).toBeInTheDocument();
    expect(screen.getByText("work_mem")).toBeInTheDocument();
    expect(screen.getByText("8MB")).toBeInTheDocument();
    expect(screen.getByText("8192 kB")).toBeInTheDocument();
    expect(
      screen.getByText(/Requested override not observed/),
    ).toBeInTheDocument();
    // Nothing to edit: no inputs, no save/add/remove controls.
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /save|add|remove|discard/i }),
    ).not.toBeInTheDocument();
  });
});

// w4/255: `"qa.a174".widgets` and `qa."a174.widgets"` are distinct tables, but
// a bare `schema.name` join printed both as `qa.a174.widgets` and used that as
// the React key — so a refresh that reordered the list left phantom rows.
describe("InsightsPanel table identities", () => {
  const size = (schema: string, name: string, sizePretty: string) => ({
    schema,
    name,
    sizePretty,
  });
  const scan = (schema: string, name: string, liveRows: number) => ({
    schema,
    name,
    seqScans: 1,
    indexScans: 0,
    liveRows,
  });

  function rowsOf(heading: string): string[][] {
    const table = screen
      .getAllByRole("table")
      .find((t) =>
        t.querySelector("th")?.parentElement?.textContent?.includes(heading),
      );
    if (!table) throw new Error(`no table with ${heading}`);
    return [...table.querySelectorAll("tbody tr")].map((tr) =>
      [...tr.querySelectorAll("td")].map((td) => td.textContent ?? ""),
    );
  }

  const sizesInitial = [
    size("qa_a174", "size_control", "240 kB"),
    size("qa.a174", "widgets", "72 kB"),
    size("qa", "a174.widgets", "72 kB"),
    size("qa_a174", "shared", "16 kB"),
    size("qa_a174", "order items 雪", "16 kB"),
    size("public", "shared", "16 kB"),
    size("qa_a174", "move_control", "8192 bytes"),
  ];
  const sizesGrown = [
    size("qa_a174", "move_control", "528 kB"),
    size("qa_a174", "size_control", "240 kB"),
    size("qa.a174", "widgets", "72 kB"),
    size("qa", "a174.widgets", "72 kB"),
    size("qa_a174", "order items 雪", "16 kB"),
    size("public", "shared", "16 kB"),
    size("qa_a174", "shared", "16 kB"),
  ];

  it("renders one distinct row per schema/table pair through reorders", () => {
    state.sizes = { database: null, tables: sizesInitial };
    const { rerender } = render(<InsightsPanel id="dpg-1" />);
    expect(rowsOf("Size")).toEqual([
      ['"qa_a174"."size_control"', "240 kB"],
      ['"qa.a174"."widgets"', "72 kB"],
      ['"qa"."a174.widgets"', "72 kB"],
      ['"qa_a174"."shared"', "16 kB"],
      ['"qa_a174"."order items 雪"', "16 kB"],
      ['"public"."shared"', "16 kB"],
      ['"qa_a174"."move_control"', "8192 bytes"],
    ]);

    // Growth moves a table to the top, then truncate moves it back.
    for (const tables of [sizesGrown, sizesInitial, sizesGrown]) {
      state.sizes = { database: null, tables };
      rerender(<InsightsPanel id="dpg-1" />);
      expect(rowsOf("Size")).toEqual(
        tables.map((t) => [`"${t.schema}"."${t.name}"`, t.sizePretty]),
      );
    }

    // Deleting a table removes exactly its row; adding one adds exactly one.
    state.sizes = {
      database: null,
      tables: [
        ...sizesGrown.filter((t) => t.schema !== "qa"),
        size("qa", "new", "8 kB"),
      ],
    };
    rerender(<InsightsPanel id="dpg-1" />);
    const labels = rowsOf("Size").map((r) => r[0]);
    expect(labels).toHaveLength(7);
    expect(labels).not.toContain('"qa"."a174.widgets"');
    expect(labels).toContain('"qa.a174"."widgets"');
    expect(labels.at(-1)).toBe('"qa"."new"');
  });

  it("keeps table-scan counts with their own pair through a reorder", () => {
    const a = scan("qa.a174", "widgets", 500);
    const b = scan("qa", "a174.widgets", 7);
    const c = scan("public", "shared", 1);
    state.tableScans = [c, a, b];
    const { rerender } = render(<InsightsPanel id="dpg-1" />);
    state.tableScans = [b, c, a];
    rerender(<InsightsPanel id="dpg-1" />);
    expect(rowsOf("Seq scans")).toEqual([
      ['"qa"."a174.widgets"', "1", "0", "7"],
      ['"public"."shared"', "1", "0", "1"],
      ['"qa.a174"."widgets"', "1", "0", "500"],
    ]);
  });

  it("doubles embedded quotes so a quoted dot cannot impersonate a separator", () => {
    state.tableScans = [scan('a"."b', "c", 1), scan("a", 'b"."c', 2)];
    render(<InsightsPanel id="dpg-1" />);
    expect(rowsOf("Seq scans").map((r) => r[0])).toEqual([
      '"a"".""b"."c"',
      '"a"."b"".""c"',
    ]);
  });
});
