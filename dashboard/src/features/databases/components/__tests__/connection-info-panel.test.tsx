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
    expect(
      screen.getByRole("button", { name: /Copy .*[Pp]assword/ }),
    ).toBeInTheDocument();
    expect(screen.queryByText("s3cretpw")).not.toBeInTheDocument();
    expect(screen.getAllByText(/•+/).length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Show password" }));
    expect(screen.getByText("s3cretpw")).toBeInTheDocument();

    // The internal string and psql command are shown (external omitted when empty).
    expect(screen.getByText("Internal connection string")).toBeInTheDocument();
    expect(screen.getByText("psql command")).toBeInTheDocument();
    expect(
      screen.queryByText("External connection string"),
    ).not.toBeInTheDocument();
  });

  // w4/586: one toggle governs the password everywhere it appears, and Copy
  // always copies the full value.
  it("masks the password inside the connection strings and psql command until shown", async () => {
    state.info = {
      password: "s3cretpw",
      internalConnectionString: "postgresql://u:s3cretpw@db-rw.default:5432/db",
      externalConnectionString:
        "postgresql://u:s3cretpw@dpg-x.db.bex.co:5432/db",
      psqlCommand: "PGPASSWORD=s3cretpw psql -h dpg-x.db.bex.co -U u db",
      serverCaCertificate: "",
      readReplicaConnectionStrings: [],
    };
    const user = userEvent.setup();
    const { container } = render(<ConnectionInfoPanel id="db" />);
    expect(container.textContent).not.toContain("s3cretpw");
    expect(
      screen.getByText("PGPASSWORD=•••••••• psql -h dpg-x.db.bex.co -U u db"),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: /Copy .*External connection string/ }),
    );
    expect(await navigator.clipboard.readText()).toBe(
      "postgresql://u:s3cretpw@dpg-x.db.bex.co:5432/db",
    );

    await user.click(screen.getByRole("button", { name: "Show password" }));
    expect(
      screen.getByText("PGPASSWORD=s3cretpw psql -h dpg-x.db.bex.co -U u db"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("postgresql://u:s3cretpw@db-rw.default:5432/db"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Hide password" }));
    expect(container.textContent).not.toContain("s3cretpw");
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

  // w2/m245: a public database serving a publicly trusted certificate returns
  // Render's URL and no CA, so nothing tells the user to download or trust one.
  it("asks for no CA download when the public certificate is publicly trusted", () => {
    state.info = {
      password: "s3cretpw",
      internalConnectionString: "postgresql://u:s3cretpw@db-rw.default:5432/db",
      externalConnectionString:
        "postgresql://u:s3cretpw@dpg-x.db.bex.co:5432/db",
      psqlCommand: "PGPASSWORD=s3cretpw psql -h dpg-x.db.bex.co -U u db",
      serverCaCertificate: "",
      readReplicaConnectionStrings: [],
    };
    render(<ConnectionInfoPanel id="dpg-x" />);
    expect(screen.getByText("External connection string")).toBeInTheDocument();
    // The password stays masked in the URL until Show password (w4/586).
    expect(
      screen.getByText("postgresql://u:••••••••@dpg-x.db.bex.co:5432/db"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Server CA certificate")).toBeNull();
    expect(
      screen.queryByRole("button", { name: /download ca certificate/i }),
    ).toBeNull();
    expect(screen.queryByText(/PGSSLROOTCERT|root\.crt|\.pem/)).toBeNull();
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

  // w4/m357: a new public database whose publicly trusted certificate is
  // still issuing (or failed) said nothing, so a copied verify-full URL just
  // failed on a stock client. The card now says why, and how to connect.
  describe("publicly trusted certificate note", () => {
    const privateCA = (externalCertificate: string): ConnectionInfoView => ({
      password: "pw",
      internalConnectionString: "postgresql://u:pw@db-rw.default:5432/db",
      externalConnectionString:
        "postgresql://u:pw@dpg-x.db.bex.co:5432/db?sslmode=verify-full",
      psqlCommand:
        "PGPASSWORD=pw psql 'host=dpg-x.db.bex.co sslmode=verify-full'",
      serverCaCertificate:
        "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n",
      externalCertificate,
      readReplicaConnectionStrings: [],
    });

    it("says the certificate is being issued while pending", () => {
      state.info = privateCA("pending");
      render(<ConnectionInfoPanel id="dpg-x" />);
      expect(
        screen.getByText("Publicly trusted certificate is being issued"),
      ).toBeInTheDocument();
      expect(screen.getByText(/sslmode=require/)).toBeInTheDocument();
      expect(screen.getByText("Server CA certificate")).toBeInTheDocument();
    });

    it("says issuance failed and the CA path works", () => {
      state.info = privateCA("failed");
      render(<ConnectionInfoPanel id="dpg-x" />);
      expect(
        screen.getByText("Publicly trusted certificate not issued yet"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/The CA path works meanwhile/),
      ).toBeInTheDocument();
      expect(
        screen.queryByText("Publicly trusted certificate is being issued"),
      ).toBeNull();
    });

    it("says nothing once trusted, or without a managed certificate", () => {
      for (const externalCertificate of ["publiclyTrusted", ""]) {
        state.info = {
          ...privateCA(externalCertificate),
          externalConnectionString: "postgresql://u:pw@dpg-x.db.bex.co:5432/db",
          serverCaCertificate: "",
        };
        const { unmount } = render(<ConnectionInfoPanel id="dpg-x" />);
        expect(screen.queryByText(/Publicly trusted certificate/)).toBeNull();
        unmount();
      }
    });
  });
});
