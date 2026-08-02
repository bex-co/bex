import fs from "fs";
import path from "path";

const routeRoot = path.resolve(process.cwd(), "app");
const routeFiles = fs
  .readdirSync(path.join(routeRoot, "(app)"))
  .filter((file) => file.endsWith(".tsx") && file !== "_layout.tsx")
  .sort();

describe("ADR048 mobile scope", () => {
  it("exposes only the supervision tabs at the app root", () => {
    expect(routeFiles).toEqual(["activity.tsx", "index.tsx", "sessions.tsx"]);
  });

  it("keeps credentials out of ordinary storage and diagnostics", () => {
    const authRoot = path.resolve(process.cwd(), "src/features/auth");
    const authText = fs
      .readdirSync(authRoot, { recursive: true })
      .filter(
        (entry) =>
          String(entry).endsWith(".ts") || String(entry).endsWith(".tsx"),
      )
      .filter((entry) => !String(entry).includes("__tests__"))
      .map((entry) =>
        fs.readFileSync(path.join(authRoot, String(entry)), "utf8"),
      )
      .join("\n");
    expect(authText).not.toContain("AsyncStorage");
    expect(authText).not.toContain("console.");
    expect(authText).not.toContain("WebView");
    expect(authText).not.toContain("clientSecret");
  });

  it("does not register destructive, configuration, editor, or shell routes", () => {
    const routeText = fs
      .readdirSync(routeRoot, { recursive: true })
      .filter((entry) => String(entry).endsWith(".tsx"))
      .map((entry) =>
        fs.readFileSync(path.join(routeRoot, String(entry)), "utf8"),
      )
      .join("\n")
      .toLowerCase();
    for (const forbidden of [
      "delete service",
      "delete workspace",
      "point-in-time recovery",
      "failover",
      "blueprint editor",
      "web shell",
    ]) {
      expect(routeText).not.toContain(forbidden);
    }
  });
});
