import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SQLConsole } from "@/features/databases/components/sql-console";
import { isReadOnlySQL } from "@/features/databases/lib/sql";

const mocks = vi.hoisted(() => ({
  execute: vi.fn(),
}));

vi.mock("@/features/databases/hooks/use-execute-database-query", () => ({
  useExecuteDatabaseQuery: () => ({ execute: mocks.execute, loading: false }),
}));

beforeEach(() => {
  mocks.execute.mockReset();
  window.sessionStorage.clear();
});

describe("SQLConsole", () => {
  it("holds execution while suspended and keeps the draft for after resume (w4/177)", async () => {
    mocks.execute.mockResolvedValue({
      columns: ["marker"],
      rows: [["29"]],
      rowCount: 1,
      truncated: false,
    });
    const user = userEvent.setup();
    const { rerender } = render(<SQLConsole id="sleepy-db" suspended />);
    const editor = screen.getByRole("textbox", { name: "SQL query" });
    await user.clear(editor);
    await user.type(editor, "SELECT 29 AS marker");

    expect(
      screen.getByText(/This database is suspended\. Resume it to run SQL/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Run query" })).toBeDisabled();
    await user.type(editor, "{Control>}{Enter}{/Control}");
    expect(mocks.execute).not.toHaveBeenCalled();

    rerender(<SQLConsole id="sleepy-db" suspended={false} />);
    expect(editor).toHaveValue("SELECT 29 AS marker");
    expect(screen.queryByText(/This database is suspended/)).toBeNull();
    await user.click(screen.getByRole("button", { name: "Run query" }));
    await waitFor(() =>
      expect(mocks.execute).toHaveBeenCalledWith("SELECT 29 AS marker", false),
    );
  });

  it("does not run an open write confirmation once the database is suspended", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<SQLConsole id="sleepy-db" />);
    const editor = screen.getByRole("textbox", { name: "SQL query" });
    await user.clear(editor);
    await user.type(editor, "DELETE FROM t");
    await user.click(screen.getByRole("button", { name: "Run query" }));
    const dialog = await screen.findByRole("alertdialog");

    rerender(<SQLConsole id="sleepy-db" suspended />);
    await user.click(within(dialog).getByRole("button", { name: /run/i }));
    expect(mocks.execute).not.toHaveBeenCalled();
  });

  it("runs read-only SQL, renders paginated rows, and recalls session history", async () => {
    const rows = Array.from({ length: 55 }, (_, index) => [
      String(index),
      `row-${index}`,
    ]);
    mocks.execute.mockResolvedValue({
      columns: ["id", "name"],
      rows,
      rowCount: 55,
      truncated: false,
    });
    const user = userEvent.setup();
    const { unmount } = render(<SQLConsole id="orders-db" />);
    const editor = screen.getByRole("textbox", { name: "SQL query" });

    await user.clear(editor);
    await user.type(editor, "SELECT 42");
    await user.click(screen.getByRole("button", { name: "Run query" }));

    await waitFor(() =>
      expect(mocks.execute).toHaveBeenCalledWith("SELECT 42", false),
    );
    expect(await screen.findByText("row-0")).toBeInTheDocument();
    expect(screen.queryByText("row-50")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(await screen.findByText("row-50")).toBeInTheDocument();
    expect(screen.getByText("Page 2 of 2")).toBeInTheDocument();

    unmount();
    render(<SQLConsole id="orders-db" />);
    const recalled = await screen.findByRole("button", { name: "SELECT 42" });
    const newEditor = screen.getByRole("textbox", { name: "SQL query" });
    await user.clear(newEditor);
    await user.type(newEditor, "SELECT 99");
    await user.click(recalled);
    expect(newEditor).toHaveValue("SELECT 42");
  });

  it("requires explicit confirmation before a non-read-only statement", async () => {
    mocks.execute.mockResolvedValue({
      columns: [],
      rows: [],
      rowCount: 3,
      truncated: false,
    });
    const user = userEvent.setup();
    render(<SQLConsole id="orders-db" />);
    const editor = screen.getByRole("textbox", { name: "SQL query" });

    await user.clear(editor);
    await user.type(editor, "DELETE FROM orders");
    await user.click(screen.getByRole("button", { name: "Run query" }));
    expect(mocks.execute).not.toHaveBeenCalled();

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText(/can change or delete database data/i),
    ).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole("button", { name: "Run statement" }),
    );

    await waitFor(() =>
      expect(mocks.execute).toHaveBeenCalledWith("DELETE FROM orders", true),
    );
    expect(await screen.findByText("3 rows affected")).toBeInTheDocument();
  });

  it("renders typed-value cells verbatim and only null as NULL (w4/m157)", async () => {
    // The GraphQL cells bex-api returns for special numerics, temporal
    // infinities, UUIDs, a native matrix and an exact numeric.
    const cells = [
      "NaN",
      "-Infinity",
      "infinity",
      "550e8400-e29b-41d4-a716-446655440000",
      "[[1,2],[3,4]]",
      '[["a",null],["NULL","b"]]',
      "12345678901234567890.123456789",
      null,
    ];
    mocks.execute.mockResolvedValue({
      columns: cells.map((_, index) => `c${index}`),
      rows: [cells],
      rowCount: 1,
      truncated: false,
    });
    const user = userEvent.setup();
    render(<SQLConsole id="orders-db" />);

    await user.click(screen.getByRole("button", { name: "Run query" }));

    const row = (await screen.findByText("NaN")).closest("tr");
    expect(row).not.toBeNull();
    const rendered = within(row as HTMLElement)
      .getAllByRole("cell")
      .map((cell) => cell.textContent);
    expect(rendered).toEqual([...cells.slice(0, -1), "NULL"]);
  });

  it("shows execution errors without swallowing them", async () => {
    mocks.execute.mockRejectedValue(new Error("bad request: SQLSTATE 42601"));
    const user = userEvent.setup();
    render(<SQLConsole id="orders-db" />);

    await user.click(screen.getByRole("button", { name: "Run query" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "bad request: SQLSTATE 42601",
    );
  });
});

describe("isReadOnlySQL", () => {
  it("recognizes safe leading keywords and conservatively confirms everything else", () => {
    expect(isReadOnlySQL("-- inspect\n SELECT 1")).toBe(true);
    expect(isReadOnlySQL("/* inspect */ SHOW work_mem")).toBe(true);
    expect(isReadOnlySQL("EXPLAIN SELECT 1")).toBe(true);
    expect(isReadOnlySQL("DELETE FROM orders")).toBe(false);
    expect(isReadOnlySQL("WITH rows AS (SELECT 1) SELECT * FROM rows")).toBe(
      false,
    );
  });
});
