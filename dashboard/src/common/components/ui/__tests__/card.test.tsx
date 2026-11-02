import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Card, CardHeader, CardTitle } from "../card";

// w4/m249: a card title is a heading by default.
describe("CardTitle", () => {
  it("is an h2 by default and keeps the title styling", () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>General</CardTitle>
        </CardHeader>
      </Card>,
    );
    const title = screen.getByRole("heading", { name: "General", level: 2 });
    expect(title).toHaveAttribute("data-slot", "card-title");
    expect(title).toHaveClass("leading-none", "font-semibold");
  });

  it("renders the explicit level, or a div when the title is only a label", () => {
    render(
      <>
        <CardTitle as="h1">Page</CardTitle>
        <CardTitle as="h3">Nested</CardTitle>
        <CardTitle as="div">Label</CardTitle>
      </>,
    );
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Page");
    expect(screen.getByRole("heading", { level: 3 })).toHaveTextContent(
      "Nested",
    );
    expect(screen.getByText("Label").tagName).toBe("DIV");
    expect(screen.queryByRole("heading", { name: "Label" })).toBeNull();
  });
});
