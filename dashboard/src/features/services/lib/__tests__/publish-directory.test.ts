import { describe, expect, it } from "vitest";
import { publishDirectoryError } from "../publish-directory";

// w21/050: the operator's checkout-escape rule for a repo-backed static site
// (lego/operator/internal/publish publish.go validate and its test, which pin
// the same joined-path vectors).
const ESCAPES = "services.publishPathEscapes";
const CONTROL = "services.publishPathControlChar";

describe("publishDirectoryError", () => {
  const repo = (rootDir: string | null) => ({ rootDir, repoBacked: true });

  it.each([
    ["dist", "examples/hello-node"],
    [".", "examples/hello-node"],
    ["./dist", ""],
    // An absolute publish path joins under the Root Directory, as path.Join does.
    ["/etc", "examples/hello-node"],
    // A `..` that stays inside the repository publishes a directory outside the
    // Root Directory: whether to refuse it is a separate decision.
    ["../static-site", "examples/hello-node"],
    ["a/../b", ""],
    // Two levels up from a two-level Root Directory is the repository root.
    ["../../x", "examples/hello-node"],
    ["", "examples/hello-node"],
  ])("accepts %j under Root Directory %j", (publishPath, rootDir) => {
    expect(publishDirectoryError(publishPath, repo(rootDir))).toBeNull();
  });

  it.each([
    ["../../../x", "examples/hello-node"],
    ["..", ""],
    ["../x", ""],
    ["a/../../x", ""],
    ["../..", "a"],
    ["  ../x  ", ""],
  ])("marks %j under Root Directory %j", (publishPath, rootDir) => {
    expect(publishDirectoryError(publishPath, repo(rootDir))).toBe(ESCAPES);
  });

  it("treats a missing Root Directory as the repository root", () => {
    expect(publishDirectoryError("../x", repo(null))).toBe(ESCAPES);
    expect(publishDirectoryError("dist", repo(null))).toBeNull();
  });

  it("refuses a control character for any site, as bex-api does", () => {
    expect(publishDirectoryError("a\u0000b", repo("x"))).toBe(CONTROL);
    expect(publishDirectoryError("a\nb", repo("x"))).toBe(CONTROL);
    expect(
      publishDirectoryError("a\rb", { rootDir: null, repoBacked: false }),
    ).toBe(CONTROL);
  });

  it("does not judge an image-backed site's path for the checkout", () => {
    const image = { rootDir: null, repoBacked: false };
    expect(publishDirectoryError("/usr/share/nginx/html", image)).toBeNull();
    expect(publishDirectoryError("../../../x", image)).toBeNull();
  });
});
