import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { EnvVarKey } from "@/features/services/types";

// Control the data/behavior the panel sees by mocking the feature hooks; keep the
// real classifyEnvVarError so the error-state routing is exercised for real.
const mockUseEnvVarKeys = vi.fn();
const mockReveal = vi.fn();
const mockSetVar = vi.fn();
const mockDeleteVar = vi.fn();
const mockDownload = vi.fn();

vi.mock("@/features/services/lib/env-export", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/features/services/lib/env-export")>();
  return {
    ...actual,
    downloadEnvFile: (...a: unknown[]) => mockDownload(...a),
  };
});

vi.mock("@/features/services/hooks/use-env-vars", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("@/features/services/hooks/use-env-vars")
    >();
  return {
    ...actual,
    useEnvVarKeys: (...a: unknown[]) => mockUseEnvVarKeys(...a),
    useRevealEnvVar: () => mockReveal,
    useEnvVarMutations: () => ({
      setVar: mockSetVar,
      deleteVar: mockDeleteVar,
      busy: false,
    }),
  };
});

import { EnvVarsPanel } from "@/features/services/components/env-vars-panel";

function keysResult(
  keys: EnvVarKey[],
  over: Partial<{ loading: boolean; error: Error | undefined }> = {},
) {
  return {
    keys,
    loading: false,
    error: undefined,
    refetch: vi.fn().mockResolvedValue(keys),
    ...over,
  };
}

beforeEach(() => {
  mockUseEnvVarKeys.mockReset();
  mockReveal.mockReset();
  mockSetVar.mockReset().mockResolvedValue(true);
  mockDeleteVar.mockReset().mockResolvedValue(true);
  mockDownload.mockReset();
});

describe("EnvVarsPanel", () => {
  it("lists keys with masked values (no value shown until revealed)", () => {
    mockUseEnvVarKeys.mockReturnValue(
      keysResult([
        { id: "FOO", key: "FOO" },
        { id: "BAR", key: "BAR" },
      ]),
    );
    render(<EnvVarsPanel serviceId="web" />);

    expect(screen.getByText("FOO")).toBeInTheDocument();
    expect(screen.getByText("BAR")).toBeInTheDocument();
    // masked, not the value
    expect(screen.getAllByText("••••••••••••")).toHaveLength(2);
    // reveal is not called on render — values are fetched only on demand
    expect(mockReveal).not.toHaveBeenCalled();
  });

  it("reveals a single value on demand via envVar(key)", async () => {
    mockReveal.mockResolvedValue("s3cret");
    mockUseEnvVarKeys.mockReturnValue(keysResult([{ id: "FOO", key: "FOO" }]));
    const user = userEvent.setup();
    render(<EnvVarsPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Show value" }));

    await waitFor(() => expect(screen.getByText("s3cret")).toBeInTheDocument());
    expect(mockReveal).toHaveBeenCalledWith("FOO");
  });

  it("renders the empty state when there are no variables", () => {
    mockUseEnvVarKeys.mockReturnValue(keysResult([]));
    render(<EnvVarsPanel serviceId="web" />);
    expect(screen.getByText("No environment variables")).toBeInTheDocument();
  });

  it("renders the unavailable (503) state when the store is unconfigured", () => {
    mockUseEnvVarKeys.mockReturnValue(
      keysResult([], { error: new Error("secret store not configured") }),
    );
    render(<EnvVarsPanel serviceId="web" />);
    expect(
      screen.getByText("Environment variables unavailable"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Export" })).toBeDisabled();
  });

  it("exports a deterministic dotenv file only after freshly revealing every value", async () => {
    mockUseEnvVarKeys.mockReturnValue(
      keysResult([
        { id: "ZED", key: "ZED" },
        { id: "ALPHA", key: "ALPHA" },
      ]),
    );
    mockReveal.mockImplementation(async (key: string) => `${key}-value`);
    const user = userEvent.setup();
    render(<EnvVarsPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Export" }));

    await waitFor(() =>
      expect(mockDownload).toHaveBeenCalledWith(
        "web.env",
        'ALPHA="ALPHA-value"\nZED="ZED-value"\n',
      ),
    );
    expect(mockReveal).toHaveBeenCalledTimes(2);
  });

  it("fails closed when any masked value cannot be revealed", async () => {
    mockUseEnvVarKeys.mockReturnValue(
      keysResult([
        { id: "VISIBLE", key: "VISIBLE" },
        { id: "MASKED", key: "MASKED" },
      ]),
    );
    mockReveal.mockImplementation(async (key: string) => {
      if (key === "MASKED") throw new Error("secret store unavailable");
      return "fresh";
    });
    const user = userEvent.setup();
    render(<EnvVarsPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Export" }));

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Export" })).toBeEnabled(),
    );
    expect(mockDownload).not.toHaveBeenCalled();
  });

  it("renders the forbidden (403) state on a permission error", () => {
    mockUseEnvVarKeys.mockReturnValue(
      keysResult([], { error: new Error("forbidden") }),
    );
    render(<EnvVarsPanel serviceId="web" />);
    expect(screen.getByText("Not authorized")).toBeInTheDocument();
  });

  it("validates the key and only calls setVar for a valid name", async () => {
    mockUseEnvVarKeys.mockReturnValue(keysResult([]));
    const user = userEvent.setup();
    render(<EnvVarsPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: /Add variable/ }));
    const keyInput = screen.getByLabelText("Key");
    const valueInput = screen.getByLabelText("Value");

    // invalid key (space) => validation message, no mutation
    await user.type(keyInput, "BAD KEY");
    await user.type(valueInput, "v");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(
      screen.getByText(/Use letters, digits and underscores/),
    ).toBeInTheDocument();
    expect(mockSetVar).not.toHaveBeenCalled();

    // fix to a valid key => setVar called
    await user.clear(keyInput);
    await user.type(keyInput, "API_KEY");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(mockSetVar).toHaveBeenCalledWith("API_KEY", "v");
  });

  it("requests server-side generation without sending a literal value", async () => {
    mockUseEnvVarKeys.mockReturnValue(keysResult([]));
    const user = userEvent.setup();
    render(<EnvVarsPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: /Add variable/ }));
    await user.type(screen.getByLabelText("Key"), "SESSION_SECRET");
    await user.click(screen.getByRole("button", { name: "Generate" }));
    expect(screen.getByLabelText("Value")).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(mockSetVar).toHaveBeenCalledWith("SESSION_SECRET", "", true);
  });

  it("deletes a variable after confirming", async () => {
    mockUseEnvVarKeys.mockReturnValue(keysResult([{ id: "FOO", key: "FOO" }]));
    const user = userEvent.setup();
    render(<EnvVarsPanel serviceId="web" />);

    // the row's trash button (aria-label "Delete") opens the confirm dialog
    await user.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Remove FOO?")).toBeInTheDocument();
    // the dialog's confirm action (also "Delete") fires the mutation
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(mockDeleteVar).toHaveBeenCalledWith("FOO"));
  });
});
