import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeyValuePersistenceModeSection } from "@/features/keyvalue/components/key-value-persistence-mode-section";

const refetch = vi.fn();
const mutate = vi.fn();
let readError: Error | undefined;
let modeData: { keyValue: { persistenceMode: string } | null } | undefined;

vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    data: modeData,
    loading: false,
    error: readError,
    refetch,
  }),
  useMutation: () => [mutate, { loading: false }],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeEach(() => {
  refetch.mockReset();
  mutate.mockReset();
  mutate.mockResolvedValue({});
  readError = undefined;
  modeData = { keyValue: { persistenceMode: "journal_snapshot" } };
});

describe("KeyValuePersistenceModeSection (real hook, wire shape)", () => {
  it("shows the saved mode from the underscored API value, not a blank", () => {
    render(<KeyValuePersistenceModeSection id="red-x" name="sessions-cache" />);
    expect(
      screen.getByRole("combobox", { name: "Persistence mode" }),
    ).toHaveTextContent("Journal + Snapshot");
  });

  it("treats the mapped mode as the baseline: Save is disabled until it changes", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValuePersistenceModeSection id="red-x" name="sessions-cache" />);

    await user.click(
      screen.getByRole("button", { name: "Edit persistence mode" }),
    );
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });

  it("keeps the selector blank for an absent read rather than guessing", () => {
    modeData = { keyValue: null };
    render(<KeyValuePersistenceModeSection id="red-x" name="sessions-cache" />);
    expect(
      screen.getByRole("combobox", { name: "Persistence mode" }),
    ).toHaveTextContent("");
  });
});

it.each([
  undefined,
  { keyValue: null },
  { keyValue: { persistenceMode: "unknown" } },
])(
  "prevents persistence changes when a failed read leaves no known mode (%j)",
  async (data) => {
    modeData = data;
    readError = new Error("Persistence read unavailable");
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValuePersistenceModeSection id="red-x" name="sessions-cache" />);
    const selector = screen.getByRole("combobox", { name: "Persistence mode" });
    expect(selector).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "Edit persistence mode" }),
    ).toBeNull();
    await user.click(selector);
    expect(
      screen.queryByRole("option", { name: "Journal + Snapshot" }),
    ).toBeNull();
    expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();
    expect(mutate).not.toHaveBeenCalled();
  },
);

it("blocks an already-open draft if a subsequent read loses the source mode", async () => {
  const user = userEvent.setup({ pointerEventsCheck: 0 });
  const { rerender } = render(
    <KeyValuePersistenceModeSection id="red-x" name="sessions-cache" />,
  );
  await user.click(
    screen.getByRole("button", { name: "Edit persistence mode" }),
  );
  await user.click(screen.getByRole("combobox", { name: "Persistence mode" }));
  await user.click(screen.getByRole("option", { name: "Snapshot only" }));
  modeData = undefined;
  readError = new Error("Persistence read unavailable");
  rerender(<KeyValuePersistenceModeSection id="red-x" name="sessions-cache" />);
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  expect(mutate).not.toHaveBeenCalled();
});
