import { renderToStaticMarkup } from "react-dom/server";
import { act, render, waitFor } from "@testing-library/react";
import {
  HeadContent,
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it } from "vitest";
import i18n from "@/i18n/init";
import {
  DASHBOARD_NAME,
  formatDashboardTitle,
  globalMetadata,
  loadRouteResource,
  normalizeDashboardOrigin,
  routeResourceTitle,
  titleHead,
  translatedTitle,
  translatedTitleHead,
} from "@/common/lib/document-head";
import ErrorPage from "@/common/root-route/error-page";
import NotFoundPage from "@/common/root-route/not-found-page";
import PendingRouteTitle from "@/common/lib/document-head/pending-route-title";

function ready<T>(resource: T) {
  return { state: "ready", resource } as const;
}

function metadataValue(
  head: ReturnType<typeof globalMetadata>,
  key: string,
): string | undefined {
  const tag = head.meta.find(
    (entry) =>
      ("name" in entry && entry.name === key) ||
      ("property" in entry && entry.property === key),
  );
  return tag && "content" in tag ? tag.content : undefined;
}

async function renderSsrHead(title: string, language: "en" | "zh" = "en") {
  function Document() {
    return (
      <html lang={language}>
        <head>
          <HeadContent />
        </head>
        <body>
          <Outlet />
        </body>
      </html>
    );
  }

  const rootRoute = createRootRoute({
    component: Document,
    head: () => globalMetadata("https://dashboard.selfhost.test", language),
  });
  const contentRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => null,
    head: () => titleHead(title),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([contentRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: {},
    isServer: true,
  });
  await router.load();

  return renderToStaticMarkup(<RouterProvider router={router} />);
}

async function renderFallbackSsr(path: "/boom" | "/missing") {
  function Document() {
    return (
      <html lang="en">
        <head>
          <HeadContent />
        </head>
        <body>
          <Outlet />
        </body>
      </html>
    );
  }

  const rootRoute = createRootRoute({
    component: Document,
    errorComponent: ErrorPage,
    notFoundComponent: NotFoundPage,
    head: () => globalMetadata("https://dashboard.selfhost.test", "en"),
  });
  const boomRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/boom",
    head: ({ match }) => translatedTitleHead("common.navSettings", match),
    loader: () => {
      throw new Error("synthetic route failure");
    },
    component: () => null,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([boomRoute]),
    history: createMemoryHistory({ initialEntries: [path] }),
    context: {},
    isServer: true,
    defaultErrorComponent: ErrorPage,
    defaultNotFoundComponent: NotFoundPage,
  });
  await router.load();

  return renderToStaticMarkup(<RouterProvider router={router} />);
}

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("dashboard title contract", () => {
  it("owns the exact Render-shaped separator and brand once", () => {
    expect(formatDashboardTitle("friendly", "Web Service")).toBe(
      "friendly ・ Web Service ・ bex Dashboard",
    );
    expect(formatDashboardTitle(" ", undefined, null)).toBe(DASHBOARD_NAME);
    expect(formatDashboardTitle("  Overview  ")).toBe(
      "Overview ・ bex Dashboard",
    );
  });

  it("uses the active locale for static and state titles", async () => {
    await i18n.changeLanguage("en");
    expect(translatedTitle("common.notFoundTitle")).toBe(
      "Page not found ・ bex Dashboard",
    );
    await i18n.changeLanguage("zh");
    expect(translatedTitle("common.notFoundTitle")).toBe(
      "页面未找到 ・ bex Dashboard",
    );
  });

  it("renders one route title in the initial SSR head", async () => {
    const html = await renderSsrHead(
      "storefront ・ Web Service ・ bex Dashboard",
    );

    expect(html).toContain(
      "<title>storefront ・ Web Service ・ bex Dashboard</title>",
    );
    expect(html.match(/<title>/g)).toHaveLength(1);
    expect(html).toContain('property="og:url"');
    expect(html).toContain("https://dashboard.selfhost.test/logo.png");
  });

  it("keeps a private route title out of composed SSR social metadata", async () => {
    const privateName = "secret-project";
    const html = await renderSsrHead(
      `${privateName} ・ Web Service ・ bex Dashboard`,
    );
    const socialMetadata = [...html.matchAll(/<meta\s[^>]*>/g)]
      .map(([tag]) => tag)
      .filter((tag) =>
        /(?:name="(?:description|twitter:)|property="og:)/.test(tag),
      )
      .join("\n");

    expect(html).toContain(`<title>${privateName} ・ Web Service`);
    expect(socialMetadata).not.toContain(privateName);
    expect(socialMetadata).not.toContain("srv-private");
  });

  it.each([
    ["/missing", "Page not found ・ bex Dashboard"],
    ["/boom", "Something went wrong ・ bex Dashboard"],
  ] as const)(
    "renders one deterministic fallback title for %s",
    async (path, expectedTitle) => {
      const html = await renderFallbackSsr(path);
      expect(html).toContain(`<title>${expectedTitle}</title>`);
      expect(html.match(/<title>/g)).toHaveLength(1);
      expect(html).not.toContain("synthetic route failure</title>");
    },
  );

  it("replaces private titles on navigation, rename, and locale changes", async () => {
    let resourceName = "private-alpha";
    function ClientDocument() {
      return (
        <>
          <HeadContent />
          <Outlet />
        </>
      );
    }
    const rootRoute = createRootRoute({
      component: ClientDocument,
      head: () => globalMetadata("https://selfhost.test", "en"),
    });
    const resourceRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/usage",
      component: () => null,
      loader: () => ready({ name: resourceName }),
      head: ({ loaderData }) =>
        titleHead(
          routeResourceTitle(loaderData, (resource) => [
            resource.name,
            "Database",
          ]),
        ),
    });
    const settingsRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/settings",
      component: () => null,
      head: () => translatedTitleHead("common.navSettings"),
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([resourceRoute, settingsRoute]),
      history: createMemoryHistory({ initialEntries: ["/usage"] }),
      context: {},
    });

    render(<RouterProvider router={router} />);
    await waitFor(() =>
      expect(document.title).toBe("private-alpha ・ Database ・ bex Dashboard"),
    );

    await act(async () => {
      await router.navigate({ to: "/settings" });
    });
    await waitFor(() =>
      expect(document.title).toBe("Settings ・ bex Dashboard"),
    );
    expect(document.head.innerHTML).not.toContain("private-alpha");

    resourceName = "private-renamed";
    await act(async () => {
      await router.navigate({ to: "/usage" });
      await router.invalidate();
    });
    await waitFor(() =>
      expect(document.title).toBe(
        "private-renamed ・ Database ・ bex Dashboard",
      ),
    );

    await act(async () => {
      await i18n.changeLanguage("zh");
      await router.navigate({ to: "/settings" });
    });
    await waitFor(() => expect(document.title).toBe("设置 ・ bex Dashboard"));
  });

  it("replaces the previous title while a dynamic route loader is pending", async () => {
    let resolveLoader!: (
      value: ReturnType<typeof ready<{ name: string }>>,
    ) => void;
    let navigation!: Promise<void>;
    function ClientDocument() {
      return (
        <>
          <HeadContent />
          <Outlet />
        </>
      );
    }
    const rootRoute = createRootRoute({
      component: ClientDocument,
      head: () => globalMetadata("https://selfhost.test", "en"),
    });
    const settingsRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/settings",
      component: () => null,
      head: () => translatedTitleHead("common.navSettings"),
    });
    const slowRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/billing",
      component: () => null,
      loader: () =>
        new Promise<ReturnType<typeof ready<{ name: string }>>>((resolve) => {
          resolveLoader = resolve;
        }),
      head: ({ loaderData }) =>
        titleHead(
          routeResourceTitle(loaderData, (resource) => [
            resource.name,
            "Database",
          ]),
        ),
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([settingsRoute, slowRoute]),
      history: createMemoryHistory({ initialEntries: ["/settings"] }),
      context: {},
      defaultPendingComponent: PendingRouteTitle,
      defaultPendingMs: 0,
      defaultPendingMinMs: 0,
    });

    render(<RouterProvider router={router} />);
    await waitFor(() =>
      expect(document.title).toBe("Settings ・ bex Dashboard"),
    );

    act(() => {
      navigation = router.navigate({ to: "/billing" });
    });
    await waitFor(() =>
      expect(document.title).toBe("Loading… ・ bex Dashboard"),
    );

    await act(async () => {
      resolveLoader(ready({ name: "orders" }));
      await navigation;
    });
    await waitFor(() =>
      expect(document.title).toBe("orders ・ Database ・ bex Dashboard"),
    );
  });
});

describe("global dashboard metadata", () => {
  it("is generic, localized, and derived from the active installation", () => {
    const privateValues = [
      "secret-project",
      "srv-private",
      "dep-private",
      "private-webhook",
      "whk-private",
      "owner@example.test",
    ];
    const enHead = globalMetadata(
      "https://dashboard.selfhost.test/private/path",
      "en",
    );
    const zhHead = globalMetadata("https://另一个.example/path", "zh");
    const serialized = JSON.stringify([enHead, zhHead]);

    expect(metadataValue(enHead, "og:url")).toBe(
      "https://dashboard.selfhost.test",
    );
    expect(metadataValue(enHead, "og:image")).toBe(
      "https://dashboard.selfhost.test/logo.png",
    );
    expect(metadataValue(enHead, "twitter:card")).toBe("summary_large_image");
    expect(metadataValue(enHead, "description")).toContain(
      "open-source, AI-native Render alternative",
    );
    expect(metadataValue(zhHead, "description")).toContain("开源");
    expect(serialized).not.toContain("dashboard.bex.co");
    for (const value of privateValues) expect(serialized).not.toContain(value);
    expect(serialized).not.toContain("canonical");
    expect(serialized).not.toContain("robots");
  });

  it("omits absolute URL tags when no trustworthy origin is available", () => {
    for (const origin of [
      undefined,
      null,
      "javascript:alert(1)",
      "not a url",
    ]) {
      const head = globalMetadata(origin, "en");
      expect(metadataValue(head, "og:url")).toBeUndefined();
      expect(metadataValue(head, "og:image")).toBeUndefined();
      expect(metadataValue(head, "twitter:image")).toBeUndefined();
    }
    expect(normalizeDashboardOrigin("https://selfhost.test/path?q=1")).toBe(
      "https://selfhost.test",
    );
  });
});

describe("dynamic route resource policy", () => {
  it("distinguishes ready, missing, and failed query results", async () => {
    await expect(
      loadRouteResource(
        async () => ({ data: { resource: { name: "human-name" } } }),
        (data) => data?.resource,
      ),
    ).resolves.toEqual({
      state: "ready",
      resource: { name: "human-name" },
    });
    await expect(
      loadRouteResource(
        async () => ({ data: { resource: null } }),
        (data) => data?.resource,
      ),
    ).resolves.toEqual({ state: "not-found" });
    await expect(
      loadRouteResource(
        async () => ({
          data: { resource: null },
          error: new Error("backend unavailable"),
        }),
        (data) => data?.resource,
      ),
    ).resolves.toEqual({ state: "error" });
    await expect(
      loadRouteResource(
        async () => {
          throw new Error("resource not found");
        },
        () => null,
      ),
    ).resolves.toEqual({ state: "not-found" });
  });

  it("never uses an opaque id in settled or fallback titles", () => {
    expect(
      routeResourceTitle(
        { state: "ready", resource: { id: "srv-private", name: "api" } },
        (resource) => [resource.name, "Web Service"],
      ),
    ).toBe("api ・ Web Service ・ bex Dashboard");
    expect(routeResourceTitle(undefined, () => [])).toBe(
      "Loading… ・ bex Dashboard",
    );
    expect(routeResourceTitle({ state: "not-found" }, () => [])).toBe(
      "Page not found ・ bex Dashboard",
    );
    expect(routeResourceTitle({ state: "error" }, () => [])).toBe(
      "Something went wrong ・ bex Dashboard",
    );
  });
});
