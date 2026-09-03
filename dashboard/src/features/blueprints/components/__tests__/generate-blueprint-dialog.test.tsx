import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GenerateBlueprintDialog } from "../generate-blueprint-dialog";

const generate = vi.fn(async () => ({
  data: {
    generateBlueprint: {
      manifest: "services:\n  - name: web\n",
      filename: "render.yaml",
    },
  },
}));
vi.mock("@apollo/client/react", () => ({
  useLazyQuery: () => [generate, { loading: false }],
}));
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
  useKeyValues: () => ({ keyValues: [] }),
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({
    groups: [{ id: "evg-1", name: "shared-config" }],
  }),
}));

beforeEach(() => generate.mockClear());

describe("GenerateBlueprintDialog", () => {
  it("disables Generate until a resource is selected, then previews the yaml", async () => {
    const user = userEvent.setup();
    render(<GenerateBlueprintDialog open onOpenChange={() => {}} />);

    const button = screen.getByRole("button", { name: /^generate$/i });
    expect(button).toBeDisabled();
    expect(
      screen.getByText(/select at least one resource/i),
    ).toBeInTheDocument();

    await user.click(screen.getByText("web"));
    await user.click(screen.getByText("app-db"));
    await user.click(screen.getByText("shared-config"));
    expect(button).toBeEnabled();
    await user.click(button);

    expect(generate).toHaveBeenCalledWith({
      variables: {
        ownerId: "tea-a",
        serviceIds: ["srv-1"],
        postgresIds: ["dpg-1"],
        keyValueIds: [],
        envGroupIds: ["evg-1"],
      },
    });
    expect(await screen.findByText(/services:/)).toBeInTheDocument();
    expect(
      screen.getByText(/secret values are never exported/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /download render\.yaml/i }),
    ).toBeInTheDocument();
  });
});
