import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiKeyPermissionsPicker } from "@/features/api-keys/components/api-key-permissions-picker";
import {
  ALL_SELECTION,
  CLEARED_SELECTION,
} from "@/features/api-keys/lib/permissions";
import type { PermissionSelection } from "@/features/api-keys/types";
import {
  ALL_LEVELS,
  CATALOG,
  READ_ONLY_LEVELS,
} from "@/features/api-keys/__tests__/catalog-fixtures";

/** The picker with its value held, as both dialogs hold it; `seen` records each change. */
function Harness({
  initial,
  seen,
}: {
  initial: PermissionSelection;
  seen: (next: PermissionSelection) => void;
}) {
  const [value, setValue] = useState(initial);
  return (
    <ApiKeyPermissionsPicker
      areas={CATALOG}
      error={undefined}
      errorMessage="unused"
      refetch={vi.fn()}
      value={value}
      onChange={(next) => {
        seen(next);
        setValue(next);
      }}
    />
  );
}

/** The radio group of the permission with this (catalog) name. */
function permission(name: string) {
  return screen.getByRole("radiogroup", { name });
}

/** The checked level of every listed permission, by its name. */
function checkedLevels() {
  return Object.fromEntries(
    screen.getAllByRole("radiogroup").map((group) => [
      document.getElementById(group.getAttribute("aria-labelledby")!)
        ?.textContent,
      within(group)
        .getAllByRole<HTMLInputElement>("radio")
        .find((radio) => radio.checked)
        ?.closest("label")?.textContent ?? null,
    ]),
  );
}

describe("ApiKeyPermissionsPicker (w9/m496)", () => {
  it("states Render's prompt and offers All and Read only, then Clear all", () => {
    render(<Harness initial={ALL_SELECTION} seen={vi.fn()} />);
    const picker = screen.getByRole("group", {
      name: "Select the permissions this API key can use.",
    });
    const presets = within(picker)
      .getAllByRole("button")
      .map((b) => b.textContent);
    expect(presets).toEqual(["All", "Read only", "Clear all"]);
  });

  it("All shows every permission at its broadest grantable level and says what All means", async () => {
    const seen = vi.fn();
    const user = userEvent.setup();
    render(<Harness initial={CLEARED_SELECTION} seen={seen} />);

    await user.click(screen.getByRole("button", { name: "All" }));

    expect(seen).toHaveBeenLastCalledWith(ALL_SELECTION);
    expect(
      screen.getByText(
        /All grants every permission the creator's member role allows/,
      ),
    ).toBeInTheDocument();
    expect(checkedLevels()).toEqual({
      "Web and private services, static sites, background workers, and cron jobs":
        "Read & write",
      "Environment variables and secret files": "Read & write",
      "Sandbox execution and file transfer": "Execute",
      // An area with nothing the viewer may grant stays listed at None.
      "Read workspace audit logs": "None",
      "Git connections and repositories": "Read",
    });
  });

  it("Read only selects Read wherever it may be granted, and nothing else", async () => {
    const seen = vi.fn();
    const user = userEvent.setup();
    render(<Harness initial={ALL_SELECTION} seen={seen} />);

    await user.click(screen.getByRole("button", { name: "Read only" }));

    expect(seen).toHaveBeenLastCalledWith({
      all: false,
      levels: READ_ONLY_LEVELS,
    });
    expect(
      within(
        permission(
          "Web and private services, static sites, background workers, and cron jobs",
        ),
      ).getByRole("radio", { name: "Read" }),
    ).toBeChecked();
    // Sandboxes offers no Read: the area is off and its rows are folded away.
    expect(
      screen.getByRole("switch", { name: "Grant Sandboxes permissions" }),
    ).not.toBeChecked();
    expect(
      screen.queryByRole("radiogroup", {
        name: "Sandbox execution and file transfer",
      }),
    ).not.toBeInTheDocument();
  });

  it("Clear all sets every permission to None and says the key can only identify itself", async () => {
    const seen = vi.fn();
    const user = userEvent.setup();
    render(<Harness initial={ALL_SELECTION} seen={seen} />);

    await user.click(screen.getByRole("button", { name: "Clear all" }));

    expect(seen).toHaveBeenLastCalledWith(CLEARED_SELECTION);
    expect(
      screen.getByText(
        "No permissions selected. The key can only identify itself.",
      ),
    ).toBeInTheDocument();
    for (const name of ["Services", "Sandboxes", "Git"]) {
      expect(
        screen.getByRole("switch", { name: `Grant ${name} permissions` }),
      ).not.toBeChecked();
    }
  });

  it("an edit after All becomes an explicit selection of what All showed", async () => {
    const seen = vi.fn();
    const user = userEvent.setup();
    render(<Harness initial={ALL_SELECTION} seen={seen} />);

    await user.click(
      within(permission("Environment variables and secret files")).getByRole(
        "radio",
        { name: "None" },
      ),
    );

    const levels: Record<string, string> = { ...ALL_LEVELS };
    delete levels.services_env;
    expect(seen).toHaveBeenLastCalledWith({ all: false, levels });
  });

  it("an area's toggle grants its permissions and keeps them listed after they return to None", async () => {
    const seen = vi.fn();
    const user = userEvent.setup();
    render(<Harness initial={CLEARED_SELECTION} seen={seen} />);

    const toggle = screen.getByRole("switch", {
      name: "Grant Services permissions",
    });
    await user.click(toggle);
    expect(seen).toHaveBeenLastCalledWith({
      all: false,
      levels: { services: "read_write", services_env: "read_write" },
    });

    for (const name of [
      "Web and private services, static sites, background workers, and cron jobs",
      "Environment variables and secret files",
    ]) {
      await user.click(
        within(permission(name)).getByRole("radio", { name: "None" }),
      );
    }
    expect(seen).toHaveBeenLastCalledWith(CLEARED_SELECTION);
    expect(toggle).not.toBeChecked();
    expect(
      permission("Environment variables and secret files"),
    ).toBeInTheDocument();
  });

  it("lists the actions the chosen level grants", async () => {
    const user = userEvent.setup();
    render(<Harness initial={readOnlyOf()} seen={vi.fn()} />);
    const group = permission("Environment variables and secret files");
    const help = document.getElementById(
      group.getAttribute("aria-describedby")!,
    )!;
    expect(help).toHaveTextContent("Read grants:");
    expect(help).toHaveTextContent(
      "View environment variables and secret files",
    );
    expect(help).not.toHaveTextContent("Set environment variable");

    await user.click(within(group).getByRole("radio", { name: "Write" }));
    const after = document.getElementById(
      group.getAttribute("aria-describedby")!,
    )!;
    expect(after).toHaveTextContent("Write grants:");
    expect(after).toHaveTextContent(
      "Set environment variable and secret file values",
    );
  });

  it("disables a level the viewer's role cannot grant and says which role can", () => {
    render(<Harness initial={ALL_SELECTION} seen={vi.fn()} />);

    const git = permission("Git connections and repositories");
    const readWrite = within(git).getByRole("radio", { name: "Read & write" });
    expect(readWrite).toBeDisabled();
    expect(readWrite).toHaveAccessibleDescription(
      "Read & write requires the Admin role.",
    );
    expect(within(git).getByRole("radio", { name: "Read" })).toBeEnabled();

    // Nothing in Audit logs is grantable: its toggle is off and disabled, and
    // its one level gives the reason.
    expect(
      screen.getByRole("switch", { name: "Grant Audit logs permissions" }),
    ).toBeDisabled();
    expect(
      within(permission("Read workspace audit logs")).getByRole("radio", {
        name: "Read",
      }),
    ).toHaveAccessibleDescription("Read requires the Admin role.");
  });

  it("shows the reason in the viewer's language, with Render's level name", async () => {
    const i18n = (await import("@/i18n/init")).default;
    await i18n.changeLanguage("zh");
    try {
      render(<Harness initial={ALL_SELECTION} seen={vi.fn()} />);
      expect(
        within(permission("Git connections and repositories")).getByRole(
          "radio",
          { name: "Read & write" },
        ),
      ).toHaveAccessibleDescription("Read & write 需要管理员角色。");
    } finally {
      await i18n.changeLanguage("en");
    }
  });

  it("shows the caller's message and a Try again in place of the picker when the catalog cannot load", async () => {
    const refetch = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(
      <ApiKeyPermissionsPicker
        areas={undefined}
        error={new Error("boom")}
        errorMessage="Couldn't load the permission list."
        refetch={refetch}
        value={ALL_SELECTION}
        onChange={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Couldn't load the permission list.",
    );
    expect(screen.queryByRole("button", { name: "All" })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("names its loading frame while the catalog loads", () => {
    render(
      <ApiKeyPermissionsPicker
        areas={undefined}
        error={undefined}
        errorMessage="unused"
        refetch={vi.fn()}
        value={ALL_SELECTION}
        onChange={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("status", { name: "Loading permissions…" }),
    ).toBeInTheDocument();
  });
});

function readOnlyOf(): PermissionSelection {
  return { all: false, levels: { ...READ_ONLY_LEVELS } };
}
