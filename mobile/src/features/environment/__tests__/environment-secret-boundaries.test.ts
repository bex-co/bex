import fs from "fs";
import path from "path";

const featureRoot = path.resolve(process.cwd(), "src/features/environment");
const productionFiles = fs
  .readdirSync(featureRoot)
  .filter((name) => /\.(?:ts|tsx)$/.test(name));
const productionSource = productionFiles
  .map((name) => fs.readFileSync(path.join(featureRoot, name), "utf8"))
  .join("\n");
const cardSource = fs.readFileSync(
  path.join(featureRoot, "environment-card.tsx"),
  "utf8",
);

describe("mobile environment secret boundaries", () => {
  it("has no ordinary persistence, clipboard, or diagnostic sink", () => {
    for (const forbidden of [
      "AsyncStorage",
      "SecureStore",
      "Clipboard",
      "console.",
    ]) {
      expect(productionSource.includes(forbidden)).toBe(false);
    }
  });

  it("keeps the one-key reveal outside Apollo cache", () => {
    expect(
      /query:\s*MobileRevealEnvVarDocument[\s\S]{0,300}fetchPolicy:\s*"no-cache"/.test(
        cardSource,
      ),
    ).toBe(true);
  });
});
