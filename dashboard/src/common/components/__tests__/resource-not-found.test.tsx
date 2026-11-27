import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n, { switchLanguage } from "@/i18n/init";
import { PlainLink } from "@/test/mocks/router-link";
import {
  AccessDenied,
  NotFoundForParentBody,
  PageNotFound,
  ResourceLoadFailed,
  ResourceNotFound,
} from "../resource-not-found";
import { useResourceNotFoundLinks } from "@/common/hooks/use-resource-state-links";

vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: PlainLink,
}));

// w3/m706: Render's ResourceNotFound, Error404, Error403 and load-error
// states (docs/render-artifacts/resource-not-found.md), drawn in place of a
// resource page whose id leads nowhere.

function buttons() {
  return screen
    .getAllByRole("link")
    .map((link) => [link.textContent, link.getAttribute("href")]);
}

function MissingService({
  projectId,
  name,
  first = [],
}: {
  projectId?: string | null;
  name?: string | null;
  first?: { label: string; to: string; params?: Record<string, string> }[];
}) {
  const links = useResourceNotFoundLinks(projectId, first);
  return (
    <ResourceNotFound kind="service" id="srv-gone" name={name} links={links} />
  );
}

describe("ResourceNotFound", () => {
  it("reads not found with Render's body and Return home alone for an id it never saw", () => {
    render(<MissingService />);

    expect(
      screen.getByRole("heading", { name: "Service not found" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading").closest("[data-resource-state]"),
    ).toHaveTextContent(
      "We couldn’t find srv-gone. It may have been deleted, or the link may be incorrect.",
    );
    expect(buttons()).toEqual([["Return home", "/"]]);
  });

  it("reads deleted with the cached name, and offers its project first", () => {
    render(<MissingService name="orders-api" projectId="prj-shop" />);

    expect(
      screen.getByRole("heading", { name: "Service deleted" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading").closest("[data-resource-state]"),
    ).toHaveTextContent("orders-api (srv-gone) was deleted.");
    expect(buttons()).toEqual([
      ["Go to project", "/project/prj-shop"],
      ["Return home", "/"],
    ]);
  });

  it("offers Go to project only when the resource sat in one", () => {
    render(<MissingService name="orders-api" projectId={null} />);
    expect(screen.queryByRole("link", { name: "Go to project" })).toBeNull();
  });

  // Render draws at most two buttons: the first link, then Return home.
  it("keeps the caller's own link first and drops Go to project behind it", () => {
    render(
      <MissingService
        projectId="prj-shop"
        first={[{ label: "View deploys", to: "/services/srv-1/deploys" }]}
      />,
    );
    expect(buttons()).toEqual([
      ["View deploys", "/services/srv-1/deploys"],
      ["Return home", "/"],
    ]);
  });

  it("titles every kind whole, a deploy and a build never as deleted", () => {
    const { rerender } = render(
      <ResourceNotFound
        kind="cronJob"
        id="srv-cron"
        name="nightly"
        links={[]}
      />,
    );
    expect(screen.getByRole("heading")).toHaveTextContent("Cron job deleted");
    rerender(<ResourceNotFound kind="sandboxGroup" id="sbg-1" links={[]} />);
    expect(screen.getByRole("heading")).toHaveTextContent(
      "Sandbox group not found",
    );
    rerender(
      <ResourceNotFound kind="deploy" id="dep-1" name="ignored" links={[]} />,
    );
    expect(screen.getByRole("heading")).toHaveTextContent("Deploy not found");
    rerender(<ResourceNotFound kind="build" id="bld-1" links={[]} />);
    expect(screen.getByRole("heading")).toHaveTextContent("Build not found");
  });

  it("draws Render's deploy body naming the parent", () => {
    render(
      <ResourceNotFound
        kind="deploy"
        id="dep-gone"
        links={[]}
        body={(id) => <NotFoundForParentBody id={id} parent="orders-api" />}
      />,
    );
    expect(
      screen.getByRole("heading").closest("[data-resource-state]"),
    ).toHaveTextContent(
      "We couldn’t find dep-gone for orders-api. The link may be incorrect.",
    );
  });

  it("copies the id from its snippet", async () => {
    const writeText = vi.fn(async () => undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    render(<MissingService />);
    await userEvent.click(screen.getByRole("button", { name: "Copy ID" }));
    expect(writeText).toHaveBeenCalledWith("srv-gone");
  });

  it("sets the tab's title to what it draws and gives the old one back", () => {
    document.title = "orders-api ・ Web Service ・ bex Dashboard";
    const { unmount } = render(<MissingService />);
    expect(document.title).toBe("Service not found ・ bex Dashboard");
    unmount();
    expect(document.title).toBe("orders-api ・ Web Service ・ bex Dashboard");
  });
});

describe("the other states", () => {
  it("PageNotFound is Render's Error404, with the page's own action when it has one", () => {
    const { rerender } = render(<PageNotFound id="dpg-gone" />);
    expect(
      screen.getByRole("heading", { name: "Page not found" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading").closest("[data-resource-state]"),
    ).toHaveTextContent("dpg-gone doesn’t exist.");
    expect(buttons()).toEqual([["Return home", "/"]]);
    rerender(
      <PageNotFound
        id="whk-gone"
        action={{ label: "Go to Webhooks", to: "/webhooks" }}
      />,
    );
    expect(buttons()).toEqual([["Go to Webhooks", "/webhooks"]]);
  });

  it("AccessDenied names the id, or the resource when it has none", () => {
    const { rerender } = render(<AccessDenied id="dpg-theirs" />);
    expect(
      screen.getByRole("heading", { name: "Access denied" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading").closest("[data-resource-state]"),
    ).toHaveTextContent("You don’t have access to dpg-theirs.");
    expect(buttons()).toEqual([["Return home", "/"]]);
    rerender(<AccessDenied />);
    expect(
      screen.getByText("You don’t have access to this resource."),
    ).toBeInTheDocument();
  });

  it("ResourceLoadFailed is Render's An error occurred with Return home", () => {
    render(<ResourceLoadFailed id="srv-1" />);
    expect(
      screen.getByRole("heading", { name: "An error occurred" }),
    ).toBeInTheDocument();
    const state = within(
      screen
        .getByRole("heading")
        .closest<HTMLElement>("[data-resource-state]")!,
    );
    expect(state.getByText(/could not be loaded\./)).toBeInTheDocument();
    expect(buttons()).toEqual([["Return home", "/"]]);
  });
});

describe("every locale", () => {
  beforeEach(async () => {
    await act(async () => {
      await switchLanguage("ja");
    });
  });
  afterEach(async () => {
    await act(async () => {
      await i18n.changeLanguage("en");
    });
  });

  // The nodes fall where the language puts them: Japanese names the id first.
  it("places the id and name where the translation puts them", () => {
    render(<MissingService name="orders-api" />);
    expect(screen.getByRole("heading")).toHaveTextContent(
      "サービスは削除されました",
    );
    expect(
      screen.getByRole("heading").closest("[data-resource-state]"),
    ).toHaveTextContent("orders-api（srv-gone）は削除されました。");
  });
});
