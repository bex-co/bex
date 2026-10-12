import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

// w4/244: at 390px a valid 30-character Key Value or Postgres name pushed the
// status badge past the viewport (document 390/416). The name/status row kept
// its content width, so the h1's `truncate` never engaged. Each detail
// header's title row — ready and pending alike, so the skeleton keeps the same
// structure — is now bounded to the header; geometry is verified in a browser.
const ROUTES = ["keyvalue.$keyValueId.tsx", "databases.$databaseId.tsx"];

function titleRows(source: string): string[] {
  const ready = source.match(
    /<div className="([^"]*)">\s*<h1 className="truncate text-xl font-semibold">/,
  )?.[1];
  const pending = source.match(
    /data-skeleton-region="resource-title"\s*className="([^"]*)"/,
  )?.[1];
  return [ready ?? "", pending ?? ""];
}

describe("datastore detail header title rows", () => {
  it.each(ROUTES)(
    "%s bounds its ready and pending name/status rows",
    (route) => {
      const source = readFileSync(
        join(import.meta.dirname, "..", route),
        "utf8",
      );
      const [ready, pending] = titleRows(source);
      for (const classes of [ready, pending]) {
        expect(classes.split(" ")).toEqual(
          expect.arrayContaining(["flex", "min-w-0", "max-w-full"]),
        );
      }
      expect(pending).toBe(ready);
    },
  );
});
