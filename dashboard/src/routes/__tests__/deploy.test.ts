import { afterEach, describe, expect, it, vi } from "vitest";
import { Route as DeployRoute } from "@/routes/deploy";

/** Where `/deploy` sends a visitor who arrived from `referrer`. */
function redirectFor(
  search: { repo?: string; path?: string },
  referrer = "",
): { to?: string; search?: Record<string, string> } {
  vi.spyOn(document, "referrer", "get").mockReturnValue(referrer);
  // Through unknown: the router's full BeforeLoadContextOptions is not
  // comparable to this one-key context on every checker (it failed `yarn
  // typecheck` on macOS while Linux CI passed), and the hook reads only search.
  const beforeLoad = DeployRoute.options.beforeLoad as unknown as (ctx: {
    search: { repo?: string; path?: string };
  }) => void;
  try {
    beforeLoad({ search });
  } catch (thrown) {
    return (thrown as { options: { to?: string; search?: never } }).options;
  }
  throw new Error("/deploy did not redirect");
}

// The Deploy-to-bex button (w8/163). Its search becomes the URL the sign-in
// bounce keeps (`requireAuth` on /blueprints/new), so a signed-out visitor
// still lands on the prefilled wizard.
describe("/deploy", () => {
  afterEach(() => vi.restoreAllMocks());

  it("prefills the linked repository and branch", () => {
    expect(
      redirectFor({ repo: "https://github.com/a/b/tree/dev", path: "x.yaml" }),
    ).toMatchObject({
      to: "/blueprints/new",
      search: { repo: "https://github.com/a/b", branch: "dev", path: "x.yaml" },
    });
  });

  it("keeps an explicit repo over the referring page", () => {
    expect(
      redirectFor({ repo: "https://github.com/a/b" }, "https://github.com/c/d")
        .search,
    ).toEqual({ repo: "https://github.com/a/b" });
  });

  // w33/003: as on Render, "the Git repository URL is determined from the
  // Referer header" when the button has no repo.
  it("deploys the referring repository page when repo is absent", () => {
    expect(
      redirectFor(
        { path: "blueprints/render.yaml" },
        "https://gitlab.example.com/a/b/tree/dev?ref=readme",
      ).search,
    ).toEqual({
      repo: "https://gitlab.example.com/a/b",
      branch: "dev",
      path: "blueprints/render.yaml",
    });
    expect(redirectFor({ repo: " " }, "https://github.com/a/b").search).toEqual(
      { repo: "https://github.com/a/b" },
    );
  });

  it("opens the empty wizard when the referrer is only an origin or names no repository", () => {
    expect(redirectFor({}, "https://github.com/").search).toEqual({});
    expect(redirectFor({}, "https://example.com/blog").search).toEqual({});
    expect(redirectFor({}).search).toEqual({});
  });
});
