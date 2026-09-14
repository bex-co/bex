import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeyValuePersistenceModeSection } from "@/features/keyvalue/components/key-value-persistence-mode-section";

const refetch = vi.fn();
let modeData: { keyValue: { persistenceMode: string } | null } | undefined;

vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    data: modeData,
    loading: false,
    error: undefined,
    refetch,
  }),
  useMutation: () => [vi.fn().mockResolvedValue({}), { loading: false }],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeEach(() => {
  refetch.mockReset();
  modeData = { keyValue: { persistenceMode: "journal_snapshot" } };
});

describe("KeyValuePersistenceModeSection (real hook, wire shape)", () => {
  it("shows the saved mode from the underscored API value, not a blank", () => {
    render(<KeyValuePersistenceModeSection id="red-x" />);
    expect(
      screen.getByRole("combobox", { name: "Persistence mode" }),
    ).toHaveTextContent("Journal + Snapshot");
  });

  it("treats the mapped mode as the baseline: Save is disabled until it changes", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValuePersistenceModeSection id="red-x" />);

    await user.click(
      screen.getByRole("button", { name: "Edit persistence mode" }),
    );
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });

  it("keeps the selector blank for an absent read rather than guessing", () => {
    modeData = { keyValue: null };
    render(<KeyValuePersistenceModeSection id="red-x" />);
    expect(
      screen.getByRole("combobox", { name: "Persistence mode" }),
    ).toHaveTextContent("");
  });
});
