import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  HeadersEditor,
  RoutesEditor,
} from "@/features/services/components/static-site-section";
import type {
  StaticHeaderView,
  StaticRouteView,
} from "@/features/services/types";
import type { StaticRuleSaveResult } from "@/features/services/hooks/use-static-site";

// w4/268: the editors seeded their draft from the saved rows only on mount.
// When a poll delivered a newer saved set (saved in another tab), an untouched
// editor kept the old rows, showed Save + Cancel, and its Save rolled the
// newer configuration back. These rerender the mounted editor with the newer
// saved props exactly as the shared Server poll does.
const OLD_HEADERS: StaticHeaderView[] = [
  { path: "/*.yaml", name: "X-QA-Overlap", value: "root-first" },
  { path: "/*", name: "x-qa-overlap", value: "other-tab-last" },
];
const NEW_HEADERS: StaticHeaderView[] = [
  OLD_HEADERS[0],
  { path: "/*", name: "x-qa-overlap", value: "other-tab-repeat" },
];
const OLD_ROUTES: StaticRouteView[] = [
  { type: "rewrite", source: "/qa-alias", destination: "/render.yaml" },
];
const NEW_ROUTES: StaticRouteView[] = [
  { type: "rewrite", source: "/qa-alias", destination: "/qa-missing.yaml" },
];

function headersEditor(
  headers: StaticHeaderView[],
  onSave: (
    rows: StaticHeaderView[],
  ) => Promise<StaticRuleSaveResult<StaticHeaderView>> = async () => ({
    ok: true,
  }),
) {
  return <HeadersEditor headers={headers} onSave={onSave} busy={false} />;
}

function values() {
  return screen
    .queryAllByRole("textbox", { name: "Value" })
    .map((el) => (el as HTMLInputElement).value);
}

const save = () => screen.getByRole("button", { name: /save headers/i });
const cancel = () => screen.queryByRole("button", { name: /cancel/i });

describe("an untouched edge-rule editor follows newer saved rules", () => {
  it("HeadersEditor adopts the newer saved set and stays clean", () => {
    const { rerender } = render(headersEditor(OLD_HEADERS));
    expect(save()).toBeDisabled();

    rerender(headersEditor(NEW_HEADERS));

    expect(values()).toEqual(["root-first", "other-tab-repeat"]);
    expect(save()).toBeDisabled();
    expect(cancel()).toBeNull();
  });

  it("RoutesEditor adopts the newer saved set and stays clean", () => {
    const onSave = vi.fn(async () => ({ ok: true }));
    const { rerender } = render(
      <RoutesEditor routes={OLD_ROUTES} onSave={onSave} busy={false} />,
    );
    rerender(<RoutesEditor routes={NEW_ROUTES} onSave={onSave} busy={false} />);

    expect(screen.getByRole("textbox", { name: "Destination" })).toHaveValue(
      "/qa-missing.yaml",
    );
    expect(screen.getByRole("button", { name: /save routes/i })).toBeDisabled();
    expect(cancel()).toBeNull();
  });

  it("follows empty→rules and rules→empty saved sets", () => {
    const { rerender } = render(headersEditor([]));
    rerender(headersEditor(NEW_HEADERS));
    expect(values()).toEqual(["root-first", "other-tab-repeat"]);
    rerender(headersEditor([]));
    expect(values()).toEqual([]);
    expect(save()).toBeDisabled();
  });
});

describe("a genuinely edited draft survives a refresh", () => {
  it("keeps the edit, its focus and its controls; Cancel adopts the latest saved rows", async () => {
    const user = userEvent.setup();
    const { rerender } = render(headersEditor(OLD_HEADERS));
    const second = screen.getAllByRole("textbox", { name: "Value" })[1];
    await user.clear(second);
    await user.type(second, "local-unsaved");

    // Unchanged saved set, then a changed one.
    rerender(headersEditor([...OLD_HEADERS]));
    rerender(headersEditor(NEW_HEADERS));

    expect(values()).toEqual(["root-first", "local-unsaved"]);
    expect(second).toHaveFocus();
    expect(save()).toBeEnabled();

    await user.click(cancel()!);
    expect(values()).toEqual(["root-first", "other-tab-repeat"]);
    expect(save()).toBeDisabled();
  });

  it("keeps an added row as an edit", async () => {
    const user = userEvent.setup();
    const { rerender } = render(headersEditor(OLD_HEADERS));
    await user.click(screen.getByRole("button", { name: /add header/i }));
    rerender(headersEditor(NEW_HEADERS));
    expect(values()).toEqual(["root-first", "other-tab-last", ""]);
  });

  it("keeps an edit typed during a pending save when the saved rows arrive", async () => {
    const user = userEvent.setup();
    let finish!: (r: StaticRuleSaveResult<StaticHeaderView>) => void;
    let submitted: StaticHeaderView[] = [];
    const onSave = vi.fn((rows: StaticHeaderView[]) => {
      submitted = rows;
      return new Promise<StaticRuleSaveResult<StaticHeaderView>>((resolve) => {
        finish = resolve;
      });
    });
    const { rerender } = render(headersEditor(OLD_HEADERS, onSave));
    const second = screen.getAllByRole("textbox", { name: "Value" })[1];
    await user.clear(second);
    await user.type(second, "submitted");
    await user.click(save());
    await user.type(second, "-newer");

    expect(submitted[1].value).toBe("submitted");
    rerender(headersEditor(submitted, onSave));
    finish({ ok: true, saved: submitted });

    expect(await screen.findByDisplayValue("submitted-newer")).toBe(second);
    expect(save()).toBeEnabled();
  });
});
