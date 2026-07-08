import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConnectionInfoPanel } from "@/features/databases/components/connection-info-panel";
import type { ConnectionInfoView } from "@/features/databases/types";

const reveal = vi.fn();
const hide = vi.fn();
const state: {
  info: ConnectionInfoView | null;
  loading: boolean;
  error: Error | undefined;
} = { info: null, loading: false, error: undefined };

vi.mock("@/features/databases/hooks/use-connection-info", () => ({
  useConnectionInfo: () => ({ ...state, reveal, hide }),
}));

beforeEach(() => {
  state.info = null;
  state.loading = false;
  state.error = undefined;
  reveal.mockReset();
  hide.mockReset();
});

describe("ConnectionInfoPanel", () => {
  it("shows only the Reveal button until asked — no credentials on screen", () => {
    render(<ConnectionInfoPanel id="db" />);
    expect(
      screen.getByRole("button", { name: /reveal connection info/i }),
    ).toBeInTheDocument();
    // Nothing sensitive is rendered before reveal.
    expect(screen.queryByText("Password")).not.toBeInTheDocument();
    expect(reveal).not.toHaveBeenCalled();
  });

  it("calls reveal on click", async () => {
    const user = userEvent.setup();
    render(<ConnectionInfoPanel id="db" />);
    await user.click(
      screen.getByRole("button", { name: /reveal connection info/i }),
    );
    expect(reveal).toHaveBeenCalledTimes(1);
  });

  it("masks the password until the show toggle, then unmasks the real value", async () => {
    state.info = {
      password: "s3cretpw",
      internalConnectionString: "postgresql://u:s3cretpw@db-rw.default:5432/db",
      externalConnectionString: "",
      psqlCommand: "PGPASSWORD=s3cretpw psql …",
    };
    const user = userEvent.setup();
    render(<ConnectionInfoPanel id="db" />);

    // Masked initially — the raw password is not on screen.
    expect(screen.getByText("Password")).toBeInTheDocument();
    expect(screen.queryByText("s3cretpw")).not.toBeInTheDocument();
    expect(screen.getByText(/•+/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Show password" }));
    expect(screen.getByText("s3cretpw")).toBeInTheDocument();

    // The internal string and psql command are shown (external omitted when empty).
    expect(screen.getByText("Internal connection string")).toBeInTheDocument();
    expect(screen.getByText("psql command")).toBeInTheDocument();
    expect(
      screen.queryByText("External connection string"),
    ).not.toBeInTheDocument();
  });

  it("renders an error state when the reveal failed", () => {
    state.error = new Error("forbidden");
    render(<ConnectionInfoPanel id="db" />);
    expect(
      screen.getByText("Couldn't load connection info"),
    ).toBeInTheDocument();
  });
});
