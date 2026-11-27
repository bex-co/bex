import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import {
  DatastoreConnectMenu,
  type DatastoreConnectMenuProps,
  type DatastoreConnectValues,
} from "@/common/components/datastore-connect-menu";

// w3/m675: Render's Connect menu on a datastore's header
// (docs/render-artifacts/postgres-dashboard.md § Header). It reads the
// values with the Info card's query each time it opens, shows Render's
// spinner, retry and Copy on each value, and the External tab's note while no
// IP rule admits traffic.

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const VALUES: DatastoreConnectValues = {
  internalConnectionString:
    "postgresql://orders_user:s3cret@dpg-main-rw.ns.svc:5432/orders",
  externalConnectionString:
    "postgresql://orders_user:s3cret@dpg-main.db.bex.co:5432/orders",
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const client = new ApolloClient({
  link: ApolloLink.empty(),
  cache: new InMemoryCache(),
});

function renderMenu(overrides: Partial<DatastoreConnectMenuProps> = {}) {
  const props: DatastoreConnectMenuProps = {
    id: "dpg-main",
    readValues: vi.fn(async () => VALUES),
    copy: {
      internalTitle: "Internal Database URL",
      externalTitle: "External Database URL",
      cliTitle: "PSQL Command",
      internalDocsHref: "https://bex.co/docs/platform/postgres#internal",
      externalDocsHref: "https://bex.co/docs/platform/postgres#external",
      copied: "Copied to clipboard",
      copyError: "Couldn't copy to clipboard",
    },
    externalOpen: true,
    cliCommand: "bex psql dpg-main",
    networkingLink: (
      <a href="/databases/dpg-main/info#access-control">Access control</a>
    ),
    disabled: false,
    ...overrides,
  };
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  render(<DatastoreConnectMenu {...props} />, { wrapper });
  return props;
}

const row = (name: string) => screen.getByRole("group", { name });

describe("a datastore's Connect menu (w3/m675)", () => {
  it("reads nothing until it opens, then shows the internal URL to copy", async () => {
    const user = userEvent.setup();
    const props = renderMenu();
    expect(props.readValues).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Connect" }));

    expect(props.readValues).toHaveBeenCalledTimes(1);
    expect(props.readValues).toHaveBeenCalledWith(client, "dpg-main");
    const internal = await waitFor(() => {
      const element = row("Internal Database URL");
      expect(element).toHaveTextContent(VALUES.internalConnectionString);
      return element;
    });
    expect(
      screen.getByRole("link", { name: "communicate internally" }),
    ).toHaveAttribute("href", "https://bex.co/docs/platform/postgres#internal");

    await user.click(
      within(internal).getByRole("button", {
        name: "Copy Internal Database URL",
      }),
    );
    // userEvent.setup() stands in for the clipboard.
    expect(await navigator.clipboard.readText()).toBe(
      VALUES.internalConnectionString,
    );
  });

  it("shows the external URL and the CLI command once an IP rule admits traffic", async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByRole("button", { name: "Connect" }));
    await user.click(screen.getByRole("tab", { name: "External" }));

    await waitFor(() =>
      expect(row("External Database URL")).toHaveTextContent(
        VALUES.externalConnectionString,
      ),
    );
    expect(
      screen.getByRole("link", { name: "services outside" }),
    ).toHaveAttribute("href", "https://bex.co/docs/platform/postgres#external");
    const cli = row("PSQL Command");
    expect(cli).toHaveTextContent("bex psql dpg-main");
    expect(screen.getByRole("link", { name: "bex CLI" })).toHaveAttribute(
      "href",
      "https://bex.co/docs/cli",
    );
    await user.click(
      within(cli).getByRole("button", { name: "Copy PSQL Command" }),
    );
    expect(await navigator.clipboard.readText()).toBe("bex psql dpg-main");
    expect(screen.queryByText(/External traffic not allowed/)).toBeNull();
  });

  it("says no external traffic is allowed, linking the IP rules, while the endpoint is closed", async () => {
    const user = userEvent.setup();
    renderMenu({ externalOpen: false });
    await user.click(screen.getByRole("button", { name: "Connect" }));
    await user.click(screen.getByRole("tab", { name: "External" }));

    const panel = screen.getByRole("tabpanel", { name: "External" });
    expect(panel).toHaveTextContent(
      "External Database URLExternal traffic not allowed. Add IP addresses in the Access control section.",
    );
    expect(
      within(panel).getByRole("link", { name: "Access control" }),
    ).toHaveAttribute("href", "/databases/dpg-main/info#access-control");
    // Neither the external URL nor the command, which would not connect.
    expect(
      screen.queryByRole("group", { name: "External Database URL" }),
    ).toBeNull();
    expect(screen.queryByRole("group", { name: "PSQL Command" })).toBeNull();
    expect(panel).not.toHaveTextContent(VALUES.externalConnectionString);
  });

  it("closes when a link inside it navigates", async () => {
    const user = userEvent.setup();
    renderMenu({ externalOpen: false });
    await user.click(screen.getByRole("button", { name: "Connect" }));
    await user.click(screen.getByRole("tab", { name: "External" }));
    const link = screen.getByRole("link", { name: "Access control" });
    link.addEventListener("click", (e) => e.preventDefault());

    await user.click(link);

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("shows a spinner and nothing to copy while the values are read", async () => {
    const user = userEvent.setup();
    const read = deferred<DatastoreConnectValues>();
    renderMenu({ readValues: () => read.promise });
    await user.click(screen.getByRole("button", { name: "Connect" }));

    const internal = row("Internal Database URL");
    expect(
      within(internal).getByRole("status", { name: "Loading…" }),
    ).toBeVisible();
    expect(within(internal).queryByRole("button")).toBeNull();
    expect(internal).not.toHaveTextContent("postgresql://");

    read.resolve(VALUES);
    await waitFor(() =>
      expect(internal).toHaveTextContent(VALUES.internalConnectionString),
    );
    expect(within(internal).queryByRole("status")).toBeNull();
  });

  it("offers Render's retry after a failed read, and reads again", async () => {
    const user = userEvent.setup();
    const readValues = vi
      .fn<DatastoreConnectMenuProps["readValues"]>()
      .mockRejectedValueOnce(new Error("not provisioned"))
      .mockResolvedValueOnce(VALUES);
    renderMenu({ readValues });
    await user.click(screen.getByRole("button", { name: "Connect" }));

    const retry = await within(row("Internal Database URL")).findByRole(
      "button",
      { name: "Loading failed. Try again?" },
    );
    expect(row("Internal Database URL")).not.toHaveTextContent("postgresql://");

    await user.click(retry);

    expect(readValues).toHaveBeenCalledTimes(2);
    await waitFor(() =>
      expect(row("Internal Database URL")).toHaveTextContent(
        VALUES.internalConnectionString,
      ),
    );
  });

  it("reads afresh each time it opens, and keeps nothing once closed", async () => {
    const user = userEvent.setup();
    const props = renderMenu();
    const connect = screen.getByRole("button", { name: "Connect" });
    await user.click(connect);
    await waitFor(() =>
      expect(row("Internal Database URL")).toHaveTextContent(
        VALUES.internalConnectionString,
      ),
    );

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(document.body).not.toHaveTextContent("s3cret");

    await user.click(connect);
    expect(props.readValues).toHaveBeenCalledTimes(2);
  });

  it("is disabled with the reason for a member who may not read connection info", async () => {
    const user = userEvent.setup();
    const props = renderMenu({
      disabled: true,
      disabledReason: "Your role can't view secrets.",
    });

    const connect = screen.getByRole("button", { name: "Connect" });
    expect(connect).toBeDisabled();
    await user.hover(connect.parentElement!);
    expect(
      (await screen.findAllByText("Your role can't view secrets.")).length,
    ).toBeGreaterThan(0);
    await user.click(connect);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(props.readValues).not.toHaveBeenCalled();
  });
});
