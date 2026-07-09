import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SecretFileName } from "@/features/services/types";

// Control the data/behavior the panel sees by mocking the feature hooks; keep the
// real classifySecretFileError so the error-state routing is exercised for real.
const mockUseSecretFileNames = vi.fn();
const mockReveal = vi.fn();
const mockSetFile = vi.fn();
const mockDeleteFile = vi.fn();

vi.mock("@/features/services/hooks/use-secret-files", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("@/features/services/hooks/use-secret-files")
    >();
  return {
    ...actual,
    useSecretFileNames: (...a: unknown[]) => mockUseSecretFileNames(...a),
    useRevealSecretFile: () => mockReveal,
    useSecretFileMutations: () => ({
      setFile: mockSetFile,
      deleteFile: mockDeleteFile,
      busy: false,
    }),
  };
});

import { SecretFilesPanel } from "@/features/services/components/secret-files-panel";

function namesResult(
  names: SecretFileName[],
  over: Partial<{ loading: boolean; error: Error | undefined }> = {},
) {
  return {
    names,
    loading: false,
    error: undefined,
    refetch: vi.fn().mockResolvedValue(names),
    ...over,
  };
}

beforeEach(() => {
  mockUseSecretFileNames.mockReset();
  mockReveal.mockReset();
  mockSetFile.mockReset().mockResolvedValue(true);
  mockDeleteFile.mockReset().mockResolvedValue(true);
});

describe("SecretFilesPanel", () => {
  it("lists file names with masked contents (nothing shown until revealed)", () => {
    mockUseSecretFileNames.mockReturnValue(
      namesResult([
        { id: "cert.pem", name: "cert.pem" },
        { id: "key.pem", name: "key.pem" },
      ]),
    );
    render(<SecretFilesPanel serviceId="web" />);

    expect(screen.getByText("cert.pem")).toBeInTheDocument();
    expect(screen.getByText("key.pem")).toBeInTheDocument();
    expect(screen.getAllByText("••••••••••••")).toHaveLength(2);
    expect(mockReveal).not.toHaveBeenCalled();
  });

  it("reveals a single file's content on demand via secretFile(name)", async () => {
    mockReveal.mockResolvedValue("-----BEGIN CERT-----");
    mockUseSecretFileNames.mockReturnValue(
      namesResult([{ id: "cert.pem", name: "cert.pem" }]),
    );
    const user = userEvent.setup();
    render(<SecretFilesPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Show value" }));

    await waitFor(() =>
      expect(screen.getByText("-----BEGIN CERT-----")).toBeInTheDocument(),
    );
    expect(mockReveal).toHaveBeenCalledWith("cert.pem");
  });

  it("renders the empty state when there are no files", () => {
    mockUseSecretFileNames.mockReturnValue(namesResult([]));
    render(<SecretFilesPanel serviceId="web" />);
    expect(screen.getByText("No secret files")).toBeInTheDocument();
  });

  it("renders the unavailable (503) state when the store is unconfigured", () => {
    mockUseSecretFileNames.mockReturnValue(
      namesResult([], { error: new Error("secret store not configured") }),
    );
    render(<SecretFilesPanel serviceId="web" />);
    expect(screen.getByText("Secret files unavailable")).toBeInTheDocument();
  });

  it("renders the forbidden (403) state on a permission error", () => {
    mockUseSecretFileNames.mockReturnValue(
      namesResult([], { error: new Error("forbidden") }),
    );
    render(<SecretFilesPanel serviceId="web" />);
    expect(screen.getByText("Not authorized")).toBeInTheDocument();
  });

  it("validates the file name and only calls setFile for a valid name", async () => {
    mockUseSecretFileNames.mockReturnValue(namesResult([]));
    const user = userEvent.setup();
    render(<SecretFilesPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: /Add secret file/ }));
    const nameInput = screen.getByLabelText("File name");
    const contentInput = screen.getByLabelText("Contents");

    // invalid name ("..") => validation message, no mutation
    await user.type(nameInput, "..");
    await user.type(contentInput, "body");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(
      screen.getByText(/Use letters, digits, dot, dash and underscore/),
    ).toBeInTheDocument();
    expect(mockSetFile).not.toHaveBeenCalled();

    // fix to a valid name => setFile called
    await user.clear(nameInput);
    await user.type(nameInput, "cert.pem");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(mockSetFile).toHaveBeenCalledWith("cert.pem", "body");
  });

  it("deletes a file after confirming", async () => {
    mockUseSecretFileNames.mockReturnValue(
      namesResult([{ id: "cert.pem", name: "cert.pem" }]),
    );
    const user = userEvent.setup();
    render(<SecretFilesPanel serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Remove cert.pem?")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    await waitFor(() =>
      expect(mockDeleteFile).toHaveBeenCalledWith("cert.pem"),
    );
  });
});
