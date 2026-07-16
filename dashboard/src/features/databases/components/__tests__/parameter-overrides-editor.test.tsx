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

import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ParameterOverridesEditor } from "@/features/databases/components/parameter-overrides-editor";

const OVERRIDES = [
  {
    name: "max_connections",
    setting: "100",
    unit: "",
    source: "configuration file",
    description: "Sets the maximum number of concurrent connections.",
  },
];

describe("ParameterOverridesEditor", () => {
  it("adds and edits rows, then sends one replace-style save", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue({ ok: true });
    render(
      <ParameterOverridesEditor
        overrides={OVERRIDES}
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
        overrides={OVERRIDES}
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
      screen.getByText("All parameters are at their defaults."),
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
        overrides={OVERRIDES}
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
