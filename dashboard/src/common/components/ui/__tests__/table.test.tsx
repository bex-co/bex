import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../table";

describe("Table", () => {
  it("should render table with overflow-x-auto container", () => {
    const { container } = render(
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Header</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>Cell</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );

    const tableContainer = container.querySelector(
      '[data-slot="table-container"]',
    );
    expect(tableContainer).toBeInTheDocument();
    expect(tableContainer).toHaveClass("overflow-x-auto");
  });

  it("should render table head without whitespace-nowrap by default", () => {
    const { container } = render(
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Header</TableHead>
          </TableRow>
        </TableHeader>
      </Table>,
    );

    const tableHead = container.querySelector('[data-slot="table-head"]');
    expect(tableHead).toBeInTheDocument();
    // Should NOT have whitespace-nowrap class
    expect(tableHead?.className).not.toContain("whitespace-nowrap");
  });

  it("should render table cell without whitespace-nowrap by default", () => {
    const { container } = render(
      <Table>
        <TableBody>
          <TableRow>
            <TableCell>Cell content</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );

    const tableCell = container.querySelector('[data-slot="table-cell"]');
    expect(tableCell).toBeInTheDocument();
    // Should NOT have whitespace-nowrap class
    expect(tableCell?.className).not.toContain("whitespace-nowrap");
  });

  it("should allow custom className on table head", () => {
    const { container } = render(
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="custom-class">Header</TableHead>
          </TableRow>
        </TableHeader>
      </Table>,
    );

    const tableHead = container.querySelector('[data-slot="table-head"]');
    expect(tableHead).toHaveClass("custom-class");
  });

  it("should allow custom className on table cell", () => {
    const { container } = render(
      <Table>
        <TableBody>
          <TableRow>
            <TableCell className="custom-class">Cell</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );

    const tableCell = container.querySelector('[data-slot="table-cell"]');
    expect(tableCell).toHaveClass("custom-class");
  });

  it("should render complete table structure", () => {
    const { container, getByText } = render(
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Column 1</TableHead>
            <TableHead>Column 2</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>Data 1</TableCell>
            <TableCell>Data 2</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );

    // Check structure exists
    expect(container.querySelector('[data-slot="table"]')).toBeInTheDocument();
    expect(
      container.querySelector('[data-slot="table-header"]'),
    ).toBeInTheDocument();
    expect(
      container.querySelector('[data-slot="table-body"]'),
    ).toBeInTheDocument();

    // Check content is rendered
    expect(getByText("Column 1")).toBeInTheDocument();
    expect(getByText("Column 2")).toBeInTheDocument();
    expect(getByText("Data 1")).toBeInTheDocument();
    expect(getByText("Data 2")).toBeInTheDocument();
  });

  // w4/685
  it("keeps words whole in head and body cells", () => {
    const { container } = render(
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>bex-canary-probes-m96</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );

    expect(container.querySelector('[data-slot="table-head"]')).toHaveClass(
      "wrap-break-word",
    );
    expect(container.querySelector('[data-slot="table-cell"]')).toHaveClass(
      "wrap-break-word",
    );
  });

  it("no table cell in the dashboard breaks words between any two letters", () => {
    const sources = import.meta.glob(
      ["/src/**/*.tsx", "!/src/**/__tests__/**", "!/src/**/*.test.tsx"],
      { query: "?raw", import: "default", eager: true },
    );
    const offenders = Object.entries(sources).flatMap(([path, source]) =>
      ((source as string).match(/<Table(?:Cell|Head)\b[^>]*>/g) ?? [])
        .filter((tag) =>
          /break-all|wrap-anywhere|overflow-wrap:anywhere/.test(tag),
        )
        .map((tag) => `${path}: ${tag}`),
    );

    expect(offenders).toEqual([]);
  });
});
