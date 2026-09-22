import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RoutesEditor,
  HeadersEditor,
} from "@/features/services/components/static-site-section";
import type {
  StaticHeaderView,
  StaticRouteView,
} from "@/features/services/types";

// w6/059: every row control in the Redirects and Headers editors must carry a
// real accessible name (the translated column-heading vocabulary) — the <th>s
// label cells, not the inputs inside them, and a hardcoded English placeholder
// must never double as the name.
describe("RoutesEditor accessibility", () => {
  it("names the row controls after the column headings, keeping placeholders as examples", () => {
    render(
      <RoutesEditor
        routes={[{ type: "redirect", source: "/old", destination: "/new" }]}
        onSave={vi.fn(async () => ({ ok: true }))}
        busy={false}
      />,
    );

    expect(screen.getByRole("combobox", { name: "Type" })).toBeInTheDocument();
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
        onSave={vi.fn(async () => ({ ok: true }))}
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

// w4/136: the clean-state half, asserted where the saved-rows PROP can be
// controlled. The editors seed their draft on mount and compute dirty against
// that prop, so after an accepted normalized save the padded draft never
// matched the canonical props again: Save stayed enabled and Cancel stayed
// present for a change that had already persisted.
describe("edge-rule editors adopt the accepted rows (w4/136)", () => {
  it("RoutesEditor returns to a clean state on a normalized save", async () => {
    const user = userEvent.setup();
    const canonical = [
      { type: "rewrite", source: "/qa-alias/*", destination: "/:splat" },
    ];
    const onSave = vi.fn(async () => ({ ok: true, saved: canonical }));

    const { rerender } = render(
      <RoutesEditor
        routes={[{ type: "rewrite", source: "/*", destination: "/index.html" }]}
        onSave={onSave}
        busy={false}
      />,
    );

    const source = screen.getByRole("textbox", { name: "Source" });
    await user.clear(source);
    await user.type(source, " /qa-alias/* ");
    const destination = screen.getByRole("textbox", { name: "Destination" });
    await user.clear(destination);
    await user.type(destination, " /:splat ");
    expect(screen.getByRole("button", { name: "Save routes" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Save routes" }));
    // The refetch the save awaited lands, so the props are canonical too.
    rerender(<RoutesEditor routes={canonical} onSave={onSave} busy={false} />);

    expect(screen.getByRole("textbox", { name: "Source" })).toHaveValue(
      "/qa-alias/*",
    );
    expect(screen.getByRole("button", { name: "Save routes" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
  });

  it("HeadersEditor returns to a clean state and never trims the value", async () => {
    const user = userEvent.setup();
    // Path is trimmed by the backend; the VALUE is preserved byte for byte.
    const canonical = [{ path: "/qa/*", name: "X-QA-Overlap", value: " on " }];
    const onSave = vi.fn(async (_rows: StaticHeaderView[]) => ({
      ok: true,
      saved: canonical,
    }));

    const { rerender } = render(
      <HeadersEditor
        headers={[{ path: "/*", name: "X-QA-Overlap", value: " on " }]}
        onSave={onSave}
        busy={false}
      />,
    );

    const path = screen.getByRole("textbox", { name: "Path" });
    await user.clear(path);
    await user.type(path, " /qa/* ");
    await user.click(screen.getByRole("button", { name: "Save headers" }));
    rerender(
      <HeadersEditor headers={canonical} onSave={onSave} busy={false} />,
    );

    expect(screen.getByRole("textbox", { name: "Path" })).toHaveValue("/qa/*");
    expect(screen.getByRole("textbox", { name: "Value" })).toHaveValue(" on ");
    expect(screen.getByRole("button", { name: "Save headers" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
    // What was submitted carried the untrimmed value too.
    expect(onSave.mock.lastCall?.[0]).toEqual([
      { path: " /qa/* ", name: "X-QA-Overlap", value: " on " },
    ]);
  });

  it("Cancel still restores the accepted rows, and add/remove order survives", async () => {
    const user = userEvent.setup();
    const routes = [
      { type: "rewrite", source: "/a/*", destination: "/1" },
      { type: "redirect", source: "/b/*", destination: "/2" },
    ];
    const onSave = vi.fn(async (rows: StaticRouteView[]) => ({
      ok: true,
      saved: rows,
    }));
    render(<RoutesEditor routes={routes} onSave={onSave} busy={false} />);

    await user.click(screen.getByRole("button", { name: "Add rule" }));
    const sources = screen.getAllByRole("textbox", { name: "Source" });
    await user.type(sources[sources.length - 1]!, "/c/*");
    const destinations = screen.getAllByRole("textbox", {
      name: "Destination",
    });
    await user.type(destinations[destinations.length - 1]!, "/3");
    await user.click(screen.getByRole("button", { name: "Save routes" }));

    expect(onSave).toHaveBeenCalledWith([
      { type: "rewrite", source: "/a/*", destination: "/1" },
      { type: "redirect", source: "/b/*", destination: "/2" },
      { type: "rewrite", source: "/c/*", destination: "/3" },
    ]);

    // A further edit, then Cancel, still restores the saved-props baseline.
    const first = screen.getAllByRole("textbox", { name: "Source" })[0]!;
    await user.clear(first);
    await user.type(first, "/changed/*");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.getAllByRole("textbox", { name: "Source" })[0]).toHaveValue(
      "/a/*",
    );
  });
});
