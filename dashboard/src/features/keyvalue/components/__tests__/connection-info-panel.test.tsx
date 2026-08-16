import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConnectionInfoPanel } from "@/features/keyvalue/components/connection-info-panel";
import type { KeyValueConnectionInfoView } from "@/features/keyvalue/types";

const reveal = vi.fn();
const hide = vi.fn();
const state: {
  info: KeyValueConnectionInfoView | null;
  loading: boolean;
  error: Error | undefined;
} = { info: null, loading: false, error: undefined };

vi.mock("@/features/keyvalue/hooks/use-connection-info", () => ({
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
  it("shows only the Reveal button until asked — nothing password-bearing on screen", () => {
    render(<ConnectionInfoPanel id="kv" />);
    expect(
      screen.getByRole("button", { name: /reveal connection info/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/redis:\/\//)).not.toBeInTheDocument();
    expect(reveal).not.toHaveBeenCalled();
  });

  it("calls reveal on click", async () => {
    const user = userEvent.setup();
    render(<ConnectionInfoPanel id="kv" />);
    await user.click(
      screen.getByRole("button", { name: /reveal connection info/i }),
    );
    expect(reveal).toHaveBeenCalledTimes(1);
  });

  it("shows the internal and CLI fields but omits the external field when not public", () => {
    state.info = {
      internalConnectionString: "redis://default:s3cr3t@kv.default.svc:6379",
      externalConnectionString: "",
      cliCommand: "redis-cli -u redis://default:s3cr3t@kv.default.svc:6379",
    };
    render(<ConnectionInfoPanel id="kv" />);

    expect(screen.getByText("Internal Key Value URL")).toBeInTheDocument();
    expect(
      screen.getByText("redis://default:s3cr3t@kv.default.svc:6379"),
    ).toBeInTheDocument();
    expect(screen.getByText("Valkey CLI command")).toBeInTheDocument();
    // Not public: the external field is replaced by the "enable public access" note.
    expect(
      screen.queryByText("External Key Value URL"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/Enable public access to get an external URL/i),
    ).toBeInTheDocument();
  });

  it("shows the external field only when the store is public", () => {
    state.info = {
      internalConnectionString: "redis://default:s3cr3t@kv.default.svc:6379",
      externalConnectionString: "rediss://default:s3cr3t@kv.kv.bex.co:6379",
      cliCommand: "redis-cli -u redis://default:s3cr3t@kv.default.svc:6379",
    };
    render(<ConnectionInfoPanel id="kv" />);

    expect(screen.getByText("External Key Value URL")).toBeInTheDocument();
    expect(
      screen.getByText("rediss://default:s3cr3t@kv.kv.bex.co:6379"),
    ).toBeInTheDocument();
  });

  it("renders an error state when the reveal failed", () => {
    state.error = new Error("forbidden");
    render(<ConnectionInfoPanel id="kv" />);
    expect(
      screen.getByText("Couldn't load connection info"),
    ).toBeInTheDocument();
  });
});
