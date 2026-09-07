import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConnectionInfoPanel } from "@/features/databases/components/connection-info-panel";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { ConnectionInfoView } from "@/features/databases/types";

const PERMISSIVE = mockCapabilities();

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
  vi.mocked(useCapabilities).mockReturnValue(PERMISSIVE);
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

  it("disables Reveal for a member without can_view_sensitive (w9/m84)", () => {
    vi.mocked(useCapabilities).mockReturnValue({
      ...PERMISSIVE,
      role: "CONTRIBUTOR",
      canViewSensitive: false,
    });
    render(<ConnectionInfoPanel id="db" />);
    expect(
      screen.getByRole("button", { name: /reveal connection info/i }),
    ).toBeDisabled();
  });

  it("masks the password until the show toggle, then unmasks the real value", async () => {
    state.info = {
      password: "s3cretpw",
      internalConnectionString: "postgresql://u:s3cretpw@db-rw.default:5432/db",
      externalConnectionString: "",
      psqlCommand: "PGPASSWORD=s3cretpw psql …",
      serverCaCertificate: "",
      readReplicaConnectionStrings: [],
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

  // w4/m95: the external string pins sslmode=verify-full against a private
  // server CA, so a public database's reveal must hand over that CA and the
  // trust-file instructions — without ever weakening the TLS mode.
  it("offers the server CA download with verify-full trust instructions", async () => {
    state.info = {
      password: "s3cretpw",
      internalConnectionString: "postgresql://u:s3cretpw@db-rw.default:5432/db",
      externalConnectionString:
        "postgresql://u:s3cretpw@db.db.bex.co:5432/db?sslmode=verify-full",
      psqlCommand: "PGPASSWORD=s3cretpw psql 'host=db.db.bex.co …'",
      serverCaCertificate:
        "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n",
      readReplicaConnectionStrings: [],
    };
    const user = userEvent.setup();
    const createObjectURL = vi.fn(() => "blob:ca");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL,
      revokeObjectURL,
    });
    // jsdom's Blob lacks .text(); capture the constructor input instead.
    const blobParts: unknown[] = [];
    vi.stubGlobal(
      "Blob",
      class {
        constructor(parts: unknown[]) {
          blobParts.push(...parts);
        }
      },
    );
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});
    try {
      render(<ConnectionInfoPanel id="dpg-x" />);

      expect(screen.getByText("Server CA certificate")).toBeInTheDocument();
      // Instructions reference the per-database filename + PGSSLROOTCERT and
      // never suggest downgrading the TLS mode.
      expect(screen.getByText(/Download dpg-x-ca\.pem/)).toBeInTheDocument();
      expect(
        screen.getByText(/PGSSLROOTCERT="\/path\/to\/dpg-x-ca\.pem"/),
      ).toBeInTheDocument();
      expect(screen.queryByText(/sslmode=disable|sslmode=require/)).toBeNull();

      await user.click(
        screen.getByRole("button", { name: /download ca certificate/i }),
      );
      expect(createObjectURL).toHaveBeenCalledTimes(1);
      expect(blobParts).toEqual([
        "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n",
      ]);
      expect(click).toHaveBeenCalledTimes(1);
      expect(revokeObjectURL).toHaveBeenCalledWith("blob:ca");
    } finally {
      click.mockRestore();
      vi.unstubAllGlobals();
    }
  });

  it("keeps a coherent panel for an internal-only database (no CA section)", () => {
    state.info = {
      password: "pw",
      internalConnectionString: "postgresql://u:pw@db-rw.default:5432/db",
      externalConnectionString: "",
      psqlCommand: "PGPASSWORD=pw psql …",
      serverCaCertificate: "",
      readReplicaConnectionStrings: [],
    };
    render(<ConnectionInfoPanel id="dpg-x" />);
    expect(screen.queryByText("Server CA certificate")).toBeNull();
    expect(screen.getByText("psql command")).toBeInTheDocument();
  });
});
