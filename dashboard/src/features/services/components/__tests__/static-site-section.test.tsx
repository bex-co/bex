import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RoutesEditor,
  HeadersEditor,
} from "@/features/services/components/static-site-section";

// w6/059: every row control in the Redirects and Headers editors must carry a
// real accessible name (the translated column-heading vocabulary) — the <th>s
// label cells, not the inputs inside them, and a hardcoded English placeholder
// must never double as the name.
describe("RoutesEditor accessibility", () => {
  it("names the row controls after the column headings, keeping placeholders as examples", () => {
    render(
      <RoutesEditor
        routes={[{ type: "redirect", source: "/old", destination: "/new" }]}
        onSave={vi.fn()}
        busy={false}
      />,
    );

    expect(
      screen.getByRole("combobox", { name: "Type" }),
    ).toBeInTheDocument();
    const source = screen.getByRole("textbox", { name: "Source" });
    expect(source).toHaveValue("/old");
    const destination = screen.getByRole("textbox", { name: "Destination" });
    expect(destination).toHaveValue("/new");
    // Placeholders stay literal path-syntax examples, not names.
    expect(source).toHaveAttribute("placeholder", "/old/*");
    expect(destination).toHaveAttribute("placeholder", "/index.html");
    expect(
      screen.getByRole("button", { name: "Remove rule" }),
    ).toBeInTheDocument();
  });
});

describe("HeadersEditor accessibility", () => {
  it("names the row controls after the column headings, keeping placeholders as examples", () => {
    render(
      <HeadersEditor
        headers={[{ path: "/*", name: "X-QA", value: "on" }]}
        onSave={vi.fn()}
        busy={false}
      />,
    );

    const path = screen.getByRole("textbox", { name: "Path" });
    expect(path).toHaveValue("/*");
    const name = screen.getByRole("textbox", { name: "Name" });
    expect(name).toHaveValue("X-QA");
    const value = screen.getByRole("textbox", { name: "Value" });
    expect(value).toHaveValue("on");
    // Placeholders stay literal header examples, not names.
    expect(name).toHaveAttribute("placeholder", "X-Frame-Options");
    expect(value).toHaveAttribute("placeholder", "DENY");
    expect(
      screen.getByRole("button", { name: "Remove header" }),
    ).toBeInTheDocument();
  });
});
