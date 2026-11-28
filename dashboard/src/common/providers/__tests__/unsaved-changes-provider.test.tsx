import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Outlet,
  RouterProvider,
  createBrowserHistory,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type RouterHistory,
} from "@tanstack/react-router";
import i18n, { ensureLanguage } from "@/i18n/init";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { editedFieldsPhrase } from "../unsaved-changes-context";
import { UnsavedChangesProvider } from "../unsaved-changes-provider";

// Render's root unsaved-changes warning (w3/118): an open settings row with a
// change it has not saved, and is not saving, holds a navigation to another
// path with "Discard unsaved changes?" and asks through beforeunload.

type Save = (value: string) => Promise<boolean>;

const FIELDS = [
  { label: "Name", value: "api" },
  { label: "Build Command", value: "make" },
  { label: "Start Command", value: "./run" },
] as const;

function renderSettings({
  onSave = async () => true,
  history = createMemoryHistory({ initialEntries: ["/settings"] }),
}: { onSave?: Save; history?: RouterHistory } = {}) {
  const root = createRootRoute({
    component: () => (
      <UnsavedChangesProvider>
        <Outlet />
      </UnsavedChangesProvider>
    ),
  });
  const settings = createRoute({
    getParentRoute: () => root,
    path: "/settings",
    component: () => (
      <>
        {FIELDS.map((field) => (
          <EditableFieldRow
            key={field.label}
            label={field.label}
            value={field.value}
            editLabel={`Edit ${field.label}`}
            onSave={onSave}
          />
        ))}
      </>
    ),
  });
  const events = createRoute({
    getParentRoute: () => root,
    path: "/events",
    component: () => <h1>Events</h1>,
  });
  const router = createRouter({
    routeTree: root.addChildren([settings, events]),
    history,
  });
  render(<RouterProvider router={router} />);
  return router;
}

async function edit(
  user: ReturnType<typeof userEvent.setup>,
  label: string,
  text: string,
) {
  await user.click(
    await screen.findByRole("button", { name: `Edit ${label}` }),
  );
  const input = screen.getByRole("textbox", { name: label });
  await user.clear(input);
  await user.type(input, text);
  return input;
}

function go(router: ReturnType<typeof renderSettings>, href: string) {
  return act(async () => {
    router.history.push(href);
  });
}

const dialog = () => screen.findByRole("alertdialog", {}, { timeout: 3000 });

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("Discard unsaved changes? (w3/118)", () => {
  it("holds a navigation away from an edited row with Render's words, naming the field in bold", async () => {
    const user = userEvent.setup();
    const router = renderSettings();
    await edit(user, "Build Command", "make build");
    await go(router, "/events");

    const modal = await dialog();
    expect(
      within(modal).getByRole("heading", { name: "Discard unsaved changes?" }),
    ).toBeInTheDocument();
    expect(modal).toHaveTextContent(
      "You've edited the Build Command field but haven't saved your changes. If you leave now, your changes will be lost.",
    );
    expect(modal.querySelector("strong")).toHaveTextContent(
      "the Build Command field",
    );
    expect(
      within(modal).getByRole("button", { name: "Keep editing" }),
    ).toBeInTheDocument();
    expect(within(modal).getByRole("button", { name: "Discard" })).toHaveClass(
      "bg-destructive",
    );
    expect(router.state.location.pathname).toBe("/settings");
    expect(screen.queryByRole("heading", { name: "Events" })).toBeNull();
  });

  it("joins two and three edited fields as Render does", async () => {
    const user = userEvent.setup();
    const router = renderSettings();
    await edit(user, "Name", "web");
    await edit(user, "Build Command", "make build");
    await go(router, "/events");
    expect((await dialog()).querySelector("strong")).toHaveTextContent(
      "the Name and Build Command fields",
    );
    await user.click(screen.getByRole("button", { name: "Keep editing" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());

    await edit(user, "Start Command", "./serve");
    await go(router, "/events");
    expect((await dialog()).querySelector("strong")).toHaveTextContent(
      "the Name, Build Command, and Start Command fields",
    );
  });

  it("Keep editing stays on the page with the edit, focused on the first edited field", async () => {
    const user = userEvent.setup();
    const router = renderSettings();
    const first = await edit(user, "Build Command", "make build");
    await edit(user, "Name", "web");
    await go(router, "/events");
    await user.click(
      within(await dialog()).getByRole("button", { name: "Keep editing" }),
    );

    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(router.state.location.pathname).toBe("/settings");
    expect(first).toHaveValue("make build");
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("web");
    await waitFor(() => expect(first).toHaveFocus());
  });

  it("Escape keeps editing, as the dialog's close does on Render", async () => {
    const user = userEvent.setup();
    const router = renderSettings();
    await edit(user, "Name", "web");
    await go(router, "/events");
    await dialog();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(router.state.location.pathname).toBe("/settings");
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("web");
  });

  it("Discard goes on and drops the edit", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn<Save>(async () => true);
    const router = renderSettings({ onSave });
    await edit(user, "Name", "web");
    await go(router, "/events");
    await user.click(
      within(await dialog()).getByRole("button", { name: "Discard" }),
    );

    expect(
      await screen.findByRole("heading", { name: "Events" }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/events");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(onSave).not.toHaveBeenCalled();

    // Back on the page, the row reads its saved value and holds nothing.
    await go(router, "/settings");
    expect(await screen.findByRole("textbox", { name: "Name" })).toHaveValue(
      "api",
    );
    await go(router, "/events");
    expect(
      await screen.findByRole("heading", { name: "Events" }),
    ).toBeInTheDocument();
  });

  it("does not hold an open row whose value is unchanged, or one changed back", async () => {
    const user = userEvent.setup();
    const router = renderSettings();
    await user.click(await screen.findByRole("button", { name: "Edit Name" }));
    await go(router, "/events");
    expect(
      await screen.findByRole("heading", { name: "Events" }),
    ).toBeInTheDocument();

    await go(router, "/settings");
    const input = await edit(user, "Name", "web");
    await user.clear(input);
    await user.type(input, "api");
    await go(router, "/events");
    expect(
      await screen.findByRole("heading", { name: "Events" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).toBeNull();
  }, 10_000);

  it("does not hold a navigation that keeps the path", async () => {
    const user = userEvent.setup();
    const router = renderSettings();
    await edit(user, "Name", "web");
    await go(router, "/settings?tab=general");
    await waitFor(() =>
      expect(router.state.location.searchStr).toBe("?tab=general"),
    );
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("web");
  });

  it("does not hold a row that is saving", async () => {
    const user = userEvent.setup();
    // The save never answers: the row stays Saving.
    const router = renderSettings({ onSave: () => new Promise(() => {}) });
    await edit(user, "Name", "web");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await go(router, "/events");
    expect(
      await screen.findByRole("heading", { name: "Events" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("holds a row whose save was refused, its edit still unsaved", async () => {
    const user = userEvent.setup();
    const router = renderSettings({ onSave: async () => false });
    await edit(user, "Name", "web");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await act(async () => {}); // the refusal lands: the row is no longer saving
    await go(router, "/events");
    expect(await dialog()).toHaveTextContent("the Name field");
  });

  it("asks through beforeunload on a reload while a row is edited, and not otherwise", async () => {
    const user = userEvent.setup();
    window.history.replaceState(null, "", "/settings");
    const history = createBrowserHistory();
    renderSettings({ history });
    const unload = () =>
      window.dispatchEvent(new Event("beforeunload", { cancelable: true }));

    await screen.findByRole("button", { name: "Edit Name" });
    expect(unload()).toBe(true);
    const input = await edit(user, "Name", "web");
    expect(unload()).toBe(false);
    await user.keyboard("{Escape}"); // the row's own Cancel
    expect(input).toHaveValue("api");
    expect(unload()).toBe(true);
    history.destroy();
  });

  it("reads the dialog in the dashboard language", async () => {
    const user = userEvent.setup();
    const router = renderSettings();
    await edit(user, "Name", "web");
    await edit(user, "Build Command", "make build");
    await act(async () => {
      await i18n.changeLanguage("zh");
    });
    await go(router, "/events");
    const modal = await dialog();
    expect(
      within(modal).getByRole("heading", { name: "放弃未保存的更改？" }),
    ).toBeInTheDocument();
    expect(modal).toHaveTextContent(
      "你编辑了Name和Build Command字段，但尚未保存更改。如果现在离开，你的更改将会丢失。",
    );
    expect(
      within(modal).getByRole("button", { name: "继续编辑" }),
    ).toBeInTheDocument();
    expect(
      within(modal).getByRole("button", { name: "放弃" }),
    ).toBeInTheDocument();
  });
});

describe("the edited fields, as each language names them (w3/118)", () => {
  beforeAll(async () => {
    for (const lang of ["de", "es", "ja", "ko", "pt", "ru"]) {
      await ensureLanguage(lang);
    }
  });

  const phrase = (lng: string, labels: string[]) =>
    editedFieldsPhrase(
      (key, params) => i18n.t(key, { ...params, lng }),
      lng,
      labels,
    );

  it.each([
    ["en", "the A field", "the A and B fields", "the A, B, and C fields"],
    ["de", "das Feld A", "die Felder A und B", "die Felder A, B und C"],
    ["es", "el campo A", "los campos A y B", "los campos A, B y C"],
    ["pt", "o campo A", "os campos A e B", "os campos A, B e C"],
    ["ja", "A フィールド", "A、B フィールド", "A、B、C フィールド"],
    ["ko", "A 필드", "A 및 B 필드", "A, B 및 C 필드"],
    ["ru", "поле A", "поля A и B", "поля A, B и C"],
    ["zh", "A字段", "A和B字段", "A、B和C字段"],
  ])("%s: one, two and three fields", (lng, one, two, three) => {
    expect(phrase(lng, ["A"])).toBe(one);
    expect(phrase(lng, ["A", "B"])).toBe(two);
    expect(phrase(lng, ["A", "B", "C"])).toBe(three);
  });

  it("ru reads five fields with the plural noun", () => {
    expect(phrase("ru", ["A", "B", "C", "D", "E"])).toBe("поля A, B, C, D и E");
  });
});
