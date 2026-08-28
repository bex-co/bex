/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ParameterOverridesEditor } from "@/features/databases/components/parameter-overrides-editor";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

// A contributor holds can_operate but not can_create (docs/ADR024) — the exact
// role the w9/m84 statement-logging gate targets.
const CONTRIBUTOR = mockCapabilities({ role: "CONTRIBUTOR", canCreate: false });
const ADMIN = mockCapabilities();

// The DECLARED set — {name, value}, no source. The editor used to take
// pg_settings rows, which is how ~48 operator-owned values became editor state
// (w6/m133).
const DECLARED = [{ name: "max_connections", value: "100" }];

describe("ParameterOverridesEditor", () => {
  // Default every test to an admin; the gating cases override per test. Keeps the
  // pre-m84 tests (which assume full permission) green.
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(ADMIN);
  });

  it("disables Save with a role reason when a contributor sets a statement-logging parameter", async () => {
    vi.mocked(useCapabilities).mockReturnValue(CONTRIBUTOR);
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue({ ok: true });
    render(
      <ParameterOverridesEditor
        parameters={[]}
        saving={false}
        onSave={onSave}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Add override" }));
    await user.type(screen.getByLabelText("Parameter 1 name"), "log_statement");
    await user.type(screen.getByLabelText("Parameter 1 value"), "all");

    expect(
      screen.getByRole("button", { name: "Save overrides" }),
    ).toBeDisabled();
  });

  it("lets a contributor save a non-logging parameter (can_operate)", async () => {
    vi.mocked(useCapabilities).mockReturnValue(CONTRIBUTOR);
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue({ ok: true });
    render(
      <ParameterOverridesEditor
        parameters={[]}
        saving={false}
        onSave={onSave}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Add override" }));
    await user.type(screen.getByLabelText("Parameter 1 name"), "work_mem");
    await user.type(screen.getByLabelText("Parameter 1 value"), "16MB");
    await user.click(screen.getByRole("button", { name: "Save overrides" }));

    expect(onSave).toHaveBeenCalledWith([{ name: "work_mem", value: "16MB" }]);
  });

  it("adds and edits rows, then sends one replace-style save", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue({ ok: true });
    render(
      <ParameterOverridesEditor
        parameters={DECLARED}
        saving={false}
        onSave={onSave}
      />,
    );

    await user.clear(screen.getByLabelText("Parameter 1 value"));
    await user.type(screen.getByLabelText("Parameter 1 value"), "200");
    await user.click(screen.getByRole("button", { name: "Add override" }));
    await user.type(screen.getByLabelText("Parameter 2 name"), "work_mem");
    await user.type(screen.getByLabelText("Parameter 2 value"), "16MB");
    await user.click(screen.getByRole("button", { name: "Save overrides" }));

    expect(onSave).toHaveBeenCalledWith([
      { name: "max_connections", value: "200" },
      { name: "work_mem", value: "16MB" },
    ]);
    expect(
      screen.getByRole("button", { name: "Save overrides" }),
    ).toBeDisabled();
  });

  it("removes the last row by saving an empty replacement", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue({ ok: true });
    render(
      <ParameterOverridesEditor
        parameters={DECLARED}
        saving={false}
        onSave={onSave}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Remove max_connections" }),
    );
    await user.click(screen.getByRole("button", { name: "Save overrides" }));

    expect(onSave).toHaveBeenCalledWith([]);
    expect(
      screen.getByText("This database sets no parameter overrides."),
    ).toBeInTheDocument();
  });

  it("shows the backend message inline and preserves the rejected draft", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue({
      ok: false,
      error: "not allowed to operate this database",
    });
    render(
      <ParameterOverridesEditor
        parameters={DECLARED}
        saving={false}
        onSave={onSave}
      />,
    );

    const value = screen.getByLabelText("Parameter 1 value");
    await user.clear(value);
    await user.type(value, "250");
    await user.click(screen.getByRole("button", { name: "Save overrides" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "not allowed to operate this database",
    );
    expect(value).toHaveValue("250");
  });
});

describe("ParameterOverridesEditor — bound to the declared set (w6/m133)", () => {
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(ADMIN);
  });

  it("shows an empty editor for a database that declares nothing", () => {
    // The live repro: a free database created five minutes earlier and never
    // configured rendered 48 editable rows, because the editor was seeded from
    // the pg_settings view instead of spec.parameters.
    render(
      <ParameterOverridesEditor
        parameters={[]}
        saving={false}
        onSave={vi.fn()}
      />,
    );

    expect(
      screen.getByText("This database sets no parameter overrides."),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Parameter 1 name")).not.toBeInTheDocument();
  });

  it("saves only the declared set, so one edit cannot capture foreign values", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue({ ok: true });
    render(
      <ParameterOverridesEditor
        parameters={[
          { name: "max_connections", value: "100" },
          { name: "work_mem", value: "8MB" },
        ]}
        saving={false}
        onSave={onSave}
      />,
    );

    // Removing one row used to submit the ~47 survivors — which, seeded from
    // pg_settings, were the operator's archive_command, restore_command and TLS
    // paths — as the tenant's own declared config.
    await user.click(screen.getByRole("button", { name: "Remove work_mem" }));
    await user.click(screen.getByRole("button", { name: "Save overrides" }));

    expect(onSave).toHaveBeenCalledWith([
      { name: "max_connections", value: "100" },
    ]);
  });

  it("offers no Source column: a declared parameter has only one source", () => {
    render(
      <ParameterOverridesEditor
        parameters={DECLARED}
        saving={false}
        onSave={vi.fn()}
      />,
    );

    // "Source" is meaningful only for pg_settings rows, and its presence here
    // was part of what made the observed config look editable.
    expect(
      screen.queryByRole("columnheader", { name: "Source" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Parameter" }),
    ).toBeInTheDocument();
  });
});
