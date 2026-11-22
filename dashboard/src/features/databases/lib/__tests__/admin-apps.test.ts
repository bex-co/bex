import { describe, expect, it } from "vitest";
import {
  generateAdminAppCredentials,
  isAdminAppId,
  randomHex,
} from "@/features/databases/lib/admin-apps";

// Render's generator (docs/render-artifacts/postgres-admin-apps.md § The
// credentials): `<app>-` and 8 hex characters, a 32-hex password, and the
// signed-in user's email for pgAdmin, which logs in by email.
describe("generateAdminAppCredentials", () => {
  it("names PgHero's login after the app with 8 random hex characters", () => {
    const { username, password } = generateAdminAppCredentials(
      "pghero",
      "ada@example.com",
    );
    expect(username).toMatch(/^pghero-[0-9a-f]{8}$/);
    expect(password).toMatch(/^[0-9a-f]{32}$/);
  });

  it("logs pgAdmin in by the signed-in user's email", () => {
    expect(
      generateAdminAppCredentials("pgadmin", "ada@example.com").username,
    ).toBe("ada@example.com");
    expect(generateAdminAppCredentials("pgadmin", undefined).username).toMatch(
      /^pgadmin-[0-9a-f]{8}$/,
    );
  });

  it("never repeats a password", () => {
    const seen = new Set(
      Array.from(
        { length: 50 },
        () => generateAdminAppCredentials("pghero", undefined).password,
      ),
    );
    expect(seen.size).toBe(50);
  });
});

describe("randomHex", () => {
  it("returns exactly the asked length, odd lengths included", () => {
    expect(randomHex(7)).toMatch(/^[0-9a-f]{7}$/);
    expect(randomHex(32)).toHaveLength(32);
  });
});

describe("isAdminAppId", () => {
  it("knows only Render's two apps", () => {
    expect(isAdminAppId("pgadmin")).toBe(true);
    expect(isAdminAppId("pghero")).toBe(true);
    expect(isAdminAppId("adminer")).toBe(false);
    expect(isAdminAppId(null)).toBe(false);
  });
});
