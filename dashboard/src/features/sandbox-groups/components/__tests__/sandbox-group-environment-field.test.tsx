import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { ProjectView } from "@/features/projects/hooks/use-projects";
import {
  SandboxGroupEnvironmentField,
  SandboxGroupEnvironmentFieldSkeleton,
} from "@/features/sandbox-groups/components/sandbox-group-environment-field";

vi.mock("@/features/environments/hooks/use-protected-placement", () => ({
  useProtectedPlacement: () => ({ restricts: () => false, reason: "" }),
}));

const environments = [
  {
    id: "evm-prod",
    projectId: "prj-shop",
    name: "Production",
    protectedStatus: "unprotected",
  },
] as unknown as EnvironmentView[];
const projects = [{ id: "prj-shop", name: "Shop" }] as ProjectView[];

const READY = {
  ready: true,
  loading: false,
  error: undefined,
  environments,
  projects,
  retry: vi.fn(),
};

/**
 * The loading state is the select's own preview (the Shape parity rule in
 * dashboard/AGENTS.md): the same wrapper, a label bar the height of the label's
 * line, and a control the select's height and width. jsdom has no layout, so
 * this pins the structural mapping a browser compare was made against: change
 * either side and this fails until the other follows.
 */
describe("SandboxGroupEnvironmentField loading state", () => {
  it("previews the ready select region: same wrapper, label line and full-width control", () => {
    const { container: ready } = render(
      <SandboxGroupEnvironmentField
        index={READY}
        value={null}
        onValueChange={vi.fn()}
      />,
    );
    const { container: loading } = render(
      <SandboxGroupEnvironmentFieldSkeleton />,
    );

    const readyRoot = ready.firstElementChild as HTMLElement;
    const loadingRoot = loading.firstElementChild as HTMLElement;
    expect(loadingRoot.className).toBe(readyRoot.className);

    // The ready region is [label, trigger]; the preview is [label bar, control].
    expect(readyRoot.children).toHaveLength(2);
    expect(loadingRoot.children).toHaveLength(2);
    const [label, trigger] = Array.from(readyRoot.children);
    const [labelBar, control] = Array.from(loadingRoot.children);
    // Label: text-sm leading-none is a 14px line, h-3.5.
    expect(label).toHaveClass("text-sm", "leading-none");
    expect(labelBar).toHaveClass("h-3.5");
    // Trigger: 38px for one line (20px line, 16px padding, 2px border; it
    // grows to wrap a long name), the full width.
    expect(trigger).toHaveClass("min-h-9", "w-full");
    expect(control).toHaveClass("h-9.5", "w-full");
  });

  it("is announced as loading, with no control to act on", () => {
    render(<SandboxGroupEnvironmentFieldSkeleton />);

    expect(
      screen.getByRole("status", { name: "Loading environments" }),
    ).toBeVisible();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });
});

describe("SandboxGroupEnvironmentField", () => {
  it("renders the skeleton while the index loads and nothing once it has no environments", () => {
    const view = render(
      <SandboxGroupEnvironmentField
        index={{ ...READY, ready: false, loading: true }}
        value={null}
        onValueChange={vi.fn()}
      />,
    );
    expect(screen.getByRole("status")).toBeVisible();

    view.rerender(
      <SandboxGroupEnvironmentField
        index={{ ...READY, environments: [] }}
        value={null}
        onValueChange={vi.fn()}
      />,
    );
    expect(view.container).toBeEmptyDOMElement();
  });

  it("keeps the select, not the skeleton, while a later re-read runs over a read index", () => {
    render(
      <SandboxGroupEnvironmentField
        index={{ ...READY, loading: true }}
        value={null}
        onValueChange={vi.fn()}
      />,
    );

    expect(screen.getByRole("combobox", { name: "Environment" })).toBeVisible();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("reports the chosen environment's id, and null for No environment", async () => {
    const onValueChange = vi.fn();
    const user = userEvent.setup();
    const view = render(
      <SandboxGroupEnvironmentField
        index={READY}
        value={null}
        onValueChange={onValueChange}
      />,
    );

    await user.click(screen.getByRole("combobox", { name: "Environment" }));
    await user.click(screen.getByRole("option", { name: "Shop / Production" }));
    expect(onValueChange).toHaveBeenLastCalledWith("evm-prod");

    view.rerender(
      <SandboxGroupEnvironmentField
        index={READY}
        value="evm-prod"
        onValueChange={onValueChange}
      />,
    );
    await user.click(screen.getByRole("combobox", { name: "Environment" }));
    await user.click(screen.getByRole("option", { name: "No environment" }));
    expect(onValueChange).toHaveBeenLastCalledWith(null);
  });

  it("announces a refusal under the select", () => {
    render(
      <SandboxGroupEnvironmentField
        index={READY}
        value="evm-prod"
        refusal="That environment no longer exists. Choose another or none."
        onValueChange={vi.fn()}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "That environment no longer exists. Choose another or none.",
    );
    expect(screen.getByRole("combobox", { name: "Environment" })).toBeVisible();
  });
});
