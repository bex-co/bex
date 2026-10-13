import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  defaultParseSearch,
} from "@tanstack/react-router";
import { parseSearchWithExactText } from "@/common/lib/search-params/exact-text";
import { parseLogSearch } from "@/features/logs/lib/log-search";
import { parseDatastoreLogSearch } from "@/features/logs/lib/datastore-log-search";
import { parseDeployListSearch } from "@/features/deploys/lib/deploy-list-search";

// w4/289: the router JSON-parses each search value, so a link searching logs
// for 9007199254740993 arrived as the number 9007199254740992 and the page
// searched for a different identifier. The app's parser keeps the digits of a
// free-text key; these go through a real router with the production
// validators.

const EXACT = "9007199254740993";

function searchSeenBy(
  validateSearch: (search: Record<string, unknown>) => object,
  href: string,
) {
  const root = createRootRoute();
  let seen: object = {};
  const page = createRoute({
    getParentRoute: () => root,
    path: "/page",
    validateSearch,
    component: function Page() {
      seen = page.useSearch();
      return <p>ready</p>;
    },
  });
  const router = createRouter({
    routeTree: root.addChildren([page]),
    history: createMemoryHistory({ initialEntries: [href] }),
    parseSearch: parseSearchWithExactText,
  });
  render(<RouterProvider router={router} />);
  return {
    router,
    read: async () => {
      await screen.findByText("ready");
      return seen as Record<string, unknown>;
    },
  };
}

describe("free-text search links keep their digits (w4/289)", () => {
  it("rounds the identifier under the default parser — the defect being fixed", () => {
    expect(defaultParseSearch(`?text=${EXACT}`)).toEqual({
      text: 9007199254740992,
    });
  });

  it.each([
    ["service Logs", parseLogSearch, "text"],
    ["datastore Logs", parseDatastoreLogSearch, "text"],
    ["deploy history", parseDeployListSearch, "q"],
  ] as const)(
    "%s receives the exact identifier from a plain link",
    async (_name, validate, key) => {
      const { read } = searchSeenBy(validate, `/page?${key}=${EXACT}`);
      expect((await read())[key]).toBe(EXACT);
    },
  );

  it("leaves the service Logs keys that are meant to be numbers alone", async () => {
    const { read } = searchSeenBy(
      parseLogSearch,
      `/page?type=app&text=${EXACT}&live=0&statusCode=404&range=12h`,
    );
    expect(await read()).toMatchObject({
      type: "app",
      text: EXACT,
      live: 0,
      statusCode: "404",
      range: "12h",
    });
  });

  it("keeps a quoted link, as the page itself writes it, unchanged", async () => {
    const { read, router } = searchSeenBy(
      parseLogSearch,
      `/page?text=%22${EXACT}%22`,
    );
    expect((await read()).text).toBe(EXACT);
    expect(router.state.location.searchStr).toBe(`?text=%22${EXACT}%22`);
  });
});

describe("parseSearchWithExactText", () => {
  it("keeps the spelling of numeric free text, including forms a number would normalize", () => {
    expect(parseSearchWithExactText("?text=1e3").text).toBe("1e3");
    expect(parseSearchWithExactText("?text=007").text).toBe("007");
    expect(parseSearchWithExactText("?text=404").text).toBe("404");
    expect(parseSearchWithExactText("?q=1.50").q).toBe("1.50");
  });

  it("parses every other key, and every non-numeric text, as the router does", () => {
    for (const search of [
      "?statusCode=404&live=0&page=2",
      "?text=hello%20world&range=1h",
      "?text=true",
      "?text=null",
      "?text=%5B1%2C2%5D",
      "?text=%7B%22a%22%3A1%7D",
      "?text=1&text=2",
      "?other=9007199254740993",
      "",
    ]) {
      expect(parseSearchWithExactText(search)).toEqual(
        defaultParseSearch(search),
      );
    }
  });
});
