import { beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { AgentSecretsCard } from "@/features/agent-environment/components/agent-secrets-card";
import type { UseAgentSecretsResult } from "@/features/agent-environment/hooks/use-agent-secrets";
import type { AgentSecretView } from "@/features/agent-environment/lib/secrets";

const hook = vi.hoisted(() => ({
  current: null as unknown as UseAgentSecretsResult,
}));

vi.mock("@/features/agent-environment/hooks/use-agent-secrets", () => ({
  useAgentSecrets: () => hook.current,
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

// The repo scope's picker offers the workspace's connected repositories.
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos: [{ fullName: "acme/api" }, { fullName: "acme/web" }],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));

const STRIPE_KEY: AgentSecretView = {
  name: "STRIPE_KEY",
  scope: "explicit",
  repo: null,
};

function setup(secrets: AgentSecretView[] = [STRIPE_KEY], loading = false) {
  const set = vi.fn(async () => undefined);
  const remove = vi.fn(async () => undefined);
  hook.current = {
    secrets,
    loading,
    error: undefined,
    refetch: vi.fn(async () => undefined),
    set,
    remove,
    saving: false,
  };
  render(<AgentSecretsCard ownerId="tea-1" />);
  return { set, remove, user: userEvent.setup() };
}

const nameField = () => screen.getByLabelText("Secret name");
const valueField = () => screen.getByLabelText("Secret value");
const save = () => screen.getByRole("button", { name: "Save" });
const scopePicker = () => screen.getByRole("combobox", { name: "Scope" });

async function pickScope(
  user: ReturnType<typeof userEvent.setup>,
  option: string,
) {
  await user.click(scopePicker());
  await user.click(await screen.findByRole("option", { name: option }));
}

/**
 * The sizing classes of each item in a form row: a fixed width, a minimum
 * one, flex-1. The primitives' own defaults (min-w-0, w-full, w-fit) are what
 * those override, so they are left out, as is the scope picker's native
 * <select>, which Radix positions out of the flow.
 */
function rowWidths(row: Element): string[] {
  const items = [...row.children].filter((child) => child.tagName !== "SELECT");
  return items.map((child) =>
    child.className
      .split(/\s+/)
      .filter(
        (c) => (/^(min-)?w-\d+$/.test(c) && c !== "min-w-0") || c === "flex-1",
      )
      .sort()
      .join(" "),
  );
}

describe("AgentSecretsCard (ADR098)", () => {
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    vi.mocked(toast.success).mockClear();
  });

  it("renders the skeleton while loading", () => {
    setup([], true);
    expect(screen.getByTestId("agent-secrets-skeleton")).toBeInTheDocument();
  });

  // The header is static copy: the skeleton renders it, so it sits where the
  // ready card puts it instead of two bars of a different height.
  it("keeps the static header while loading", () => {
    setup([], true);
    const skeleton = screen.getByTestId("agent-secrets-skeleton");
    expect(skeleton).toHaveTextContent("Agent secrets");
    expect(skeleton.querySelector("[data-slot=card-title]")).not.toBeNull();
    expect(
      skeleton.querySelector("[data-slot=card-description]"),
    ).not.toBeNull();
  });

  // The help line wraps on narrow screens: the skeleton reserves it as real
  // text, both variants in one cell, so the loaded card doesn't grow.
  it("reserves the help line as text at its taller variant", () => {
    setup([], true);
    const help = screen.getByTestId("secrets-help-skeleton");
    expect(help).toHaveClass("grid", "text-xs");
    expect(help).toHaveAttribute("aria-hidden");
    const [admin, member] = Array.from(help.children);
    expect(admin).toHaveClass("col-start-1", "row-start-1");
    expect(member).toHaveClass("invisible", "col-start-1", "row-start-1");
    expect(admin.textContent).not.toBe("");
    expect(member.textContent).not.toBe("");
  });

  it("lists names only and saves a masked value, then empties the field", async () => {
    const { user, set } = setup();
    expect(screen.getByText("STRIPE_KEY")).toBeInTheDocument();
    expect(valueField()).toHaveAttribute("type", "password");
    expect(valueField()).toHaveAttribute("autocomplete", "new-password");
    await user.type(nameField(), "SENTRY_DSN");
    await user.type(valueField(), "https://secret@sentry.example/1");
    await user.click(save());
    // A new secret without a picked scope takes the server's default
    // (workspace), which the form showed.
    expect(scopePicker()).toHaveTextContent("All sessions");
    expect(set).toHaveBeenCalledWith("SENTRY_DSN", {
      value: "https://secret@sentry.example/1",
      scope: undefined,
      repo: undefined,
    });
    expect(toast.success).toHaveBeenCalledWith("Saved SENTRY_DSN");
    expect(valueField()).toHaveValue("");
    expect(nameField()).toHaveValue("");
  });

  it("refuses reserved and malformed names before saving", async () => {
    const { user, set } = setup([]);
    expect(screen.getByText("No agent secrets yet.")).toBeInTheDocument();
    await user.type(nameField(), "BEX_AGENT_PROMPT");
    await user.type(valueField(), "x");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "bex sets this environment variable itself",
    );
    expect(save()).toBeDisabled();
    await user.clear(nameField());
    await user.type(nameField(), "has-dash");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Use letters, digits and _",
    );
    expect(set).not.toHaveBeenCalled();
  });

  it("maps a server limit refusal", async () => {
    const { user, set } = setup();
    set.mockRejectedValueOnce(codedGraphQLError("AGENT_SECRET_LIMIT_EXCEEDED"));
    await user.type(nameField(), "ANOTHER");
    await user.type(valueField(), "v");
    await user.click(save());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "reached the limit",
    );
  });

  it("deletes after confirmation", async () => {
    const { user, remove } = setup();
    await user.click(screen.getByRole("button", { name: "Delete STRIPE_KEY" }));
    await user.click(screen.getByRole("button", { name: "Delete secret" }));
    expect(remove).toHaveBeenCalledWith("STRIPE_KEY");
  });

  it("is read-only for a member without can_manage", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    setup();
    expect(screen.getByText("STRIPE_KEY")).toBeInTheDocument();
    expect(nameField()).toBeDisabled();
    expect(valueField()).toBeDisabled();
    expect(save()).toBeDisabled();
    expect(scopePicker()).toBeDisabled();
    expect(screen.getByTestId("agent-secret-scope")).toHaveTextContent(
      "Only when selected",
    );
    expect(
      screen.queryByRole("button", { name: "Delete STRIPE_KEY" }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Edit STRIPE_KEY" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "Only workspace admins can add, change or delete agent secrets.",
      ),
    ).toBeInTheDocument();
  });
  // ADR098 D9: each row says which sessions receive the secret.
  it("shows each secret's scope", () => {
    setup([
      { name: "ORG_TOKEN", scope: "workspace", repo: null },
      { name: "API_DEPLOY_KEY", scope: "repo", repo: "acme/api" },
      STRIPE_KEY,
    ]);
    const badges = screen
      .getAllByTestId("agent-secret-scope")
      .map((badge) => badge.textContent);
    expect(badges).toEqual([
      "All sessions",
      "Repository: acme/api",
      "Only when selected",
    ]);
  });

  it("saves a new secret with a picked scope and repository", async () => {
    const { user, set } = setup([]);
    await user.type(nameField(), "API_DEPLOY_KEY");
    await user.type(valueField(), "k-1");
    await pickScope(user, "One repository");
    expect(save()).toBeDisabled();
    const repo = screen.getByRole("combobox", { name: "Repository" });
    await user.type(repo, "acme");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Enter the repository as owner/name.",
    );
    expect(save()).toBeDisabled();
    await user.type(repo, "/api");
    await user.click(save());
    expect(set).toHaveBeenCalledWith("API_DEPLOY_KEY", {
      value: "k-1",
      scope: "repo",
      repo: "acme/api",
    });
  });

  // Changing only the scope never asks for the value again: the write omits
  // it and the server keeps the stored one.
  it("changes an existing secret's scope without its value", async () => {
    const { user, set } = setup([
      STRIPE_KEY,
      { name: "ORG_TOKEN", scope: "workspace", repo: null },
    ]);
    await user.click(screen.getByRole("button", { name: "Edit STRIPE_KEY" }));
    expect(nameField()).toHaveValue("STRIPE_KEY");
    expect(valueField()).toHaveValue("");
    expect(valueField()).toHaveAttribute("placeholder", "Keep current value");
    expect(scopePicker()).toHaveTextContent("Only when selected");
    // Nothing changed yet, so there is nothing to save.
    expect(save()).toBeDisabled();
    await pickScope(user, "All sessions");
    await user.click(save());
    expect(set).toHaveBeenCalledWith("STRIPE_KEY", {
      value: undefined,
      scope: "workspace",
      repo: undefined,
    });
    expect(toast.success).toHaveBeenCalledWith("Saved STRIPE_KEY");
    expect(nameField()).toHaveValue("");
  });

  // Rotating by typing an existing name keeps that secret's scope: the form
  // shows it and the write leaves it out.
  it("keeps an existing secret's scope when only its value is rotated", async () => {
    const { user, set } = setup([
      { name: "API_DEPLOY_KEY", scope: "repo", repo: "acme/api" },
    ]);
    await user.type(nameField(), "API_DEPLOY_KEY");
    expect(scopePicker()).toHaveTextContent("One repository");
    expect(screen.getByRole("combobox", { name: "Repository" })).toHaveValue(
      "acme/api",
    );
    await user.type(valueField(), "k-2");
    await user.click(save());
    expect(set).toHaveBeenCalledWith("API_DEPLOY_KEY", {
      value: "k-2",
      scope: undefined,
      repo: undefined,
    });
  });

  it("maps a server scope refusal", async () => {
    const { user, set } = setup();
    set.mockRejectedValueOnce(codedGraphQLError("AGENT_SECRET_SCOPE_INVALID"));
    await user.click(screen.getByRole("button", { name: "Edit STRIPE_KEY" }));
    await pickScope(user, "All sessions");
    await user.click(save());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Enter the repository as owner/name.",
    );
  });

  // The skeleton previews the ready form row for row: name and value, then
  // the scope picker and Save, at the same widths (dashboard/AGENTS.md).
  it("draws the form rows at the ready card's widths", () => {
    setup([], true);
    const skeletonRows = [
      ...screen
        .getByTestId("agent-secrets-skeleton")
        .querySelectorAll(".border-t > .flex"),
    ].map(rowWidths);
    cleanup();
    setup([]);
    const readyRows = [
      ...screen
        .getByTestId("agent-secrets-card")
        .querySelectorAll("form > .flex"),
    ].map(rowWidths);
    expect(readyRows).toEqual([
      ["w-48", "flex-1 min-w-48"],
      // The scope picker, then Save at its label's width (4rem).
      ["w-44", ""],
    ]);
    expect(skeletonRows).toEqual([
      ["w-48", "flex-1 min-w-48"],
      ["w-44", "w-16"],
    ]);
  });
});
