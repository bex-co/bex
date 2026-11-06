import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { AgentSecretsCard } from "@/features/agent-environment/components/agent-secrets-card";
import type { UseAgentSecretsResult } from "@/features/agent-environment/hooks/use-agent-secrets";

const hook = vi.hoisted(() => ({
  current: null as unknown as UseAgentSecretsResult,
}));

vi.mock("@/features/agent-environment/hooks/use-agent-secrets", () => ({
  useAgentSecrets: () => hook.current,
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function setup(names: string[] = ["STRIPE_KEY"], loading = false) {
  const set = vi.fn(async () => undefined);
  const remove = vi.fn(async () => undefined);
  hook.current = {
    names,
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

describe("AgentSecretsCard (ADR098)", () => {
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    vi.mocked(toast.success).mockClear();
  });

  it("renders the skeleton while loading", () => {
    setup([], true);
    expect(screen.getByTestId("agent-secrets-skeleton")).toBeInTheDocument();
  });

  it("lists names only and saves a masked value, then empties the field", async () => {
    const { user, set } = setup();
    expect(screen.getByText("STRIPE_KEY")).toBeInTheDocument();
    expect(valueField()).toHaveAttribute("type", "password");
    expect(valueField()).toHaveAttribute("autocomplete", "new-password");
    await user.type(nameField(), "SENTRY_DSN");
    await user.type(valueField(), "https://secret@sentry.example/1");
    await user.click(save());
    expect(set).toHaveBeenCalledWith(
      "SENTRY_DSN",
      "https://secret@sentry.example/1",
    );
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
    expect(
      screen.queryByRole("button", { name: "Delete STRIPE_KEY" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "Only workspace admins can add or delete agent secrets.",
      ),
    ).toBeInTheDocument();
  });
});
