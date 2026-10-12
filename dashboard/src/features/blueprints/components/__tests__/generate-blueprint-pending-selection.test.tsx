import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { GenerateBlueprintDialog } from "../generate-blueprint-dialog";

// w4/267: unchecking a resource while Generate was pending was accepted, then
// the older request's manifest (still containing that resource) opened, while
// Back showed it unchecked. The real Apollo lazy query runs against a link
// whose response the test releases by hand, so the pending window is real.
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-a" }),
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({ services: [{ id: "srv-1", name: "web" }] }),
}));
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => ({ databases: [{ id: "dpg-1", name: "app-db" }] }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => ({ keyValues: [{ id: "red-1", name: "cache" }] }),
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({ groups: [{ id: "evg-1", name: "shared-雪" }] }),
}));

type Settle = (result: { data?: unknown; errors?: unknown[] }) => void;
let requests: { variables: Record<string, unknown>; settle: Settle }[] = [];

function renderDialog() {
  const link = new ApolloLink(
    (operation) =>
      new Observable((sub) => {
        requests.push({
          variables: operation.variables,
          settle: (result) => {
            sub.next(result as never);
            sub.complete();
          },
        });
      }),
  );
  const client = new ApolloClient({ link, cache: new InMemoryCache() });
  render(
    <ApolloProvider client={client}>
      <GenerateBlueprintDialog open onOpenChange={() => {}} />
    </ApolloProvider>,
  );
}

function manifestFor(variables: Record<string, unknown>) {
  return {
    data: {
      generateBlueprint: {
        __typename: "GeneratedBlueprint",
        manifest: `envGroupIds: ${JSON.stringify(variables.envGroupIds)}\n`,
        filename: "render.yaml",
      },
    },
  };
}

const ALL = ["web", "app-db", "cache", "shared-雪"];

function checkbox(name: string) {
  return screen
    .getByText(name)
    .closest("label")!
    .querySelector('[role="checkbox"]') as HTMLElement;
}

beforeEach(() => {
  requests = [];
});

describe("GenerateBlueprintDialog while generating", () => {
  it("locks every list's selection to what was submitted, then Back keeps it", async () => {
    const user = userEvent.setup();
    renderDialog();
    for (const name of ALL) await user.click(screen.getByText(name));
    await user.click(screen.getByRole("button", { name: /^generate$/i }));
    await waitFor(() => expect(requests).toHaveLength(1));

    for (const name of ALL) {
      expect(checkbox(name)).toBeDisabled();
      await user.click(screen.getByText(name));
      checkbox(name).focus();
      await user.keyboard(" ");
      expect(checkbox(name)).toHaveAttribute("aria-checked", "true");
    }

    await act(async () =>
      requests[0].settle(manifestFor(requests[0].variables)),
    );
    expect(await screen.findByText(/envGroupIds: \["evg-1"\]/)).toBeVisible();

    await user.click(screen.getByRole("button", { name: /back/i }));
    for (const name of ALL) {
      expect(checkbox(name)).toBeEnabled();
      expect(checkbox(name)).toHaveAttribute("aria-checked", "true");
    }

    // Editing is back: drop the group and regenerate without it.
    await user.click(screen.getByText("shared-雪"));
    await user.click(screen.getByRole("button", { name: /^generate$/i }));
    await waitFor(() => expect(requests).toHaveLength(2));
    expect(requests[1].variables.envGroupIds).toEqual([]);
  });

  it("keeps the submitted selection editable for a retry after an error", async () => {
    const user = userEvent.setup();
    renderDialog();
    await user.click(screen.getByText("web"));
    await user.click(screen.getByText("shared-雪"));
    await user.click(screen.getByRole("button", { name: /^generate$/i }));
    await waitFor(() => expect(requests).toHaveLength(1));
    expect(checkbox("shared-雪")).toBeDisabled();

    await act(async () =>
      requests[0].settle({ errors: [{ message: "boom" }] }),
    );

    await waitFor(() => expect(checkbox("shared-雪")).toBeEnabled());
    expect(checkbox("web")).toHaveAttribute("aria-checked", "true");
    expect(checkbox("shared-雪")).toHaveAttribute("aria-checked", "true");
    await user.click(screen.getByText("shared-雪"));
    expect(checkbox("shared-雪")).toHaveAttribute("aria-checked", "false");
    await user.click(screen.getByRole("button", { name: /^generate$/i }));
    await waitFor(() => expect(requests).toHaveLength(2));
    expect(requests[1].variables).toMatchObject({
      serviceIds: ["srv-1"],
      envGroupIds: [],
    });
  });
});
