import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConnectionInfoPanel } from "@/features/keyvalue/components/connection-info-panel";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { KeyValueConnectionInfoView } from "@/features/keyvalue/types";

const PERMISSIVE = mockCapabilities();

const reveal = vi.fn();
const hide = vi.fn();
const state: {
  info: KeyValueConnectionInfoView | null;
  loading: boolean;
  error: Error | undefined;
} = { info: null, loading: false, error: undefined };

const enable = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-enable-key-value-internal-auth", () => ({
  useEnableKeyValueInternalAuth: () => ({ enable, loading: false }),
}));

vi.mock("@/features/keyvalue/hooks/use-connection-info", () => ({
  useConnectionInfo: () => ({ ...state, reveal, hide }),
}));

beforeEach(() => {
  state.info = null;
  state.loading = false;
  state.error = undefined;
  reveal.mockReset();
  hide.mockReset();
  enable.mockReset();
  vi.mocked(useCapabilities).mockReturnValue(PERMISSIVE);
});

describe("ConnectionInfoPanel", () => {
  it("shows only the Reveal button until asked — nothing password-bearing on screen", () => {
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic={false}
      />,
    );
    expect(
      screen.getByRole("button", { name: /reveal connection info/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/redis:\/\//)).not.toBeInTheDocument();
    expect(reveal).not.toHaveBeenCalled();
  });

  it("calls reveal on click", async () => {
    const user = userEvent.setup();
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic={false}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: /reveal connection info/i }),
    );
    expect(reveal).toHaveBeenCalledTimes(1);
  });

  it("disables Reveal for a member without can_view_sensitive — the URI is never fetched or rendered (w1/m76)", async () => {
    vi.mocked(useCapabilities).mockReturnValue({
      ...PERMISSIVE,
      role: "CONTRIBUTOR",
      canViewSensitive: false,
    });
    const user = userEvent.setup();
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic={false}
      />,
    );

    const revealButton = screen.getByRole("button", {
      name: /reveal connection info/i,
    });
    expect(revealButton).toBeDisabled();
    // A click on the disabled control never fires the fetch, and no
    // password-bearing redis:// URI ever reaches the screen.
    await user.click(revealButton);
    expect(reveal).not.toHaveBeenCalled();
    expect(screen.queryByText(/redis:\/\//)).not.toBeInTheDocument();
  });

  it("keeps Reveal enabled with can_view_sensitive, matching the databases panel", () => {
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic={false}
      />,
    );
    expect(
      screen.getByRole("button", { name: /reveal connection info/i }),
    ).toBeEnabled();
  });

  it("shows the internal and CLI fields but omits the external field when not public", () => {
    state.info = {
      internalConnectionString: "redis://default:s3cr3t@kv.default.svc:6379",
      externalConnectionString: "",
      cliCommand: "redis-cli -u redis://default:s3cr3t@kv.default.svc:6379",
    };
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic={false}
      />,
    );

    expect(screen.getByText("Internal Key Value URL")).toBeInTheDocument();
    expect(
      screen.getByText("redis://default:s3cr3t@kv.default.svc:6379"),
    ).toBeInTheDocument();
    expect(screen.getByText("Valkey CLI command")).toBeInTheDocument();
    // Not public: the external field is replaced by a note naming the control
    // that actually publishes the store — adding an inbound IP rule under
    // Networking (w4/m116). The old copy promised an "enable public access"
    // control that did not exist anywhere in the UI.
    expect(
      screen.queryByText("External Key Value URL"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/Add an inbound IP rule under Networking/i),
    ).toBeInTheDocument();
  });

  // w9/m521: the external URL is answered from the create, as on Render, and
  // shown at once; the note under it says the endpoint is not open yet.
  it("shows a public store's external URL with a pending note while its TLS listener opens", () => {
    state.info = {
      internalConnectionString: "redis://kv:6379",
      externalConnectionString: "rediss://kv:s3cr3t@kv.kv.bex.co:6379",
      cliCommand:
        " REDISCLI_AUTH=s3cr3t valkey-cli --user kv -h kv.kv.bex.co --sni kv.kv.bex.co -p 6379 --tls",
      externalCertificate: "pending",
    };
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic
      />,
    );
    expect(screen.getByText("External Key Value URL")).toBeInTheDocument();
    expect(
      screen.getByText("rediss://kv:s3cr3t@kv.kv.bex.co:6379"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/External access is being enabled/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/certificate failed/i),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Add an inbound IP rule under Networking/i),
    ).not.toBeInTheDocument();
  });

  // w4/m357 (rounds 254/304): issuance failed for hours and the card kept
  // promising "usually within a minute". It now says the URL does not connect
  // yet, and why, under the URL itself.
  it("shows a public store's external URL with a failed note when its certificate failed", () => {
    state.info = {
      internalConnectionString: "redis://kv:6379",
      externalConnectionString: "rediss://kv:s3cr3t@kv.kv.bex.co:6379",
      cliCommand:
        " REDISCLI_AUTH=s3cr3t valkey-cli --user kv -h kv.kv.bex.co --sni kv.kv.bex.co -p 6379 --tls",
      externalCertificate: "failed",
    };
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic
      />,
    );
    expect(
      screen.getByText("rediss://kv:s3cr3t@kv.kv.bex.co:6379"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/certificate failed, and bex retries it automatically/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/External access is being enabled/i),
    ).not.toBeInTheDocument();
  });

  it("adds no note under a public store's open external URL", () => {
    state.info = {
      internalConnectionString: "redis://kv:6379",
      externalConnectionString: "rediss://kv:s3cr3t@kv.kv.bex.co:6379",
      cliCommand:
        " REDISCLI_AUTH=s3cr3t valkey-cli --user kv -h kv.kv.bex.co --sni kv.kv.bex.co -p 6379 --tls",
      externalCertificate: "publiclyTrusted",
    };
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic
      />,
    );
    expect(
      screen.getByText("rediss://kv:s3cr3t@kv.kv.bex.co:6379"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/External access is being enabled/i),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/certificate failed/i),
    ).not.toBeInTheDocument();
  });

  // An older bex-api withheld the URL until the listener opened (w8/237).
  it("says a public store's external URL is still being enabled when the answer has none", () => {
    state.info = {
      internalConnectionString: "redis://kv:6379",
      externalConnectionString: "",
      cliCommand: " valkey-cli -h kv -p 6379",
    };
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic
      />,
    );
    expect(
      screen.queryByText("External Key Value URL"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/External access is being enabled/i),
    ).toBeInTheDocument();
  });

  it("shows the external field only when the store is public", () => {
    state.info = {
      internalConnectionString: "redis://default:s3cr3t@kv.default.svc:6379",
      externalConnectionString: "rediss://default:s3cr3t@kv.kv.bex.co:6379",
      cliCommand: "redis-cli -u redis://default:s3cr3t@kv.default.svc:6379",
    };
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic={false}
      />,
    );

    expect(screen.getByText("External Key Value URL")).toBeInTheDocument();
    expect(
      screen.getByText("rediss://default:s3cr3t@kv.kv.bex.co:6379"),
    ).toBeInTheDocument();
  });

  it("renders an error state when the reveal failed", () => {
    state.error = new Error("forbidden");
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic={false}
      />,
    );
    expect(
      screen.getByText("Couldn't load connection info"),
    ).toBeInTheDocument();
  });

  // w5/m268: Render's Info → Connections "Enable Internal Authentication",
  // behind a confirmation that names what breaks; enable-only.
  it("enables internal authentication only after the confirmation", async () => {
    const user = userEvent.setup();
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic={false}
      />,
    );
    expect(
      screen.getByText("Internal connections don't require a password."),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Enable Internal Authentication" }),
    );
    expect(enable).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(/Existing connections .* will break/);
    await user.click(
      within(dialog).getByRole("button", {
        name: "Enable Internal Authentication",
      }),
    );
    expect(enable).toHaveBeenCalledWith("kv", "cache");
  });

  it("offers no control once internal authentication is on", () => {
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth
        isPublic={false}
      />,
    );
    expect(
      screen.getByText("Internal connections require the password."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Enable Internal Authentication" }),
    ).not.toBeInTheDocument();
  });

  it("disables enabling for a role that cannot change the store", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canCreate: false }),
    );
    render(
      <ConnectionInfoPanel
        id="kv"
        name="cache"
        internalAuth={false}
        isPublic={false}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Enable Internal Authentication" }),
    ).toBeDisabled();
  });
});
