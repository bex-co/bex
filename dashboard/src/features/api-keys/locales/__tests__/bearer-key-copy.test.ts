import { describe, expect, it } from "vitest";

import de from "../de";
import en from "../en";
import es from "../es";
import ja from "../ja";
import ko from "../ko";
import pt from "../pt";
import ru from "../ru";
import zh from "../zh";

/**
 * An API key is a bearer token (w7/m426, docs/ADR012-auth.md §8e): the user
 * sends the one string as `Authorization: Bearer <key>`, as on Render. Until
 * then every catalog said the opposite ("This is an OAuth2 client_credentials
 * client, not a bearer token. Exchange it at …") and that sentence must not
 * come back in any language. The client_credentials exchange is still there,
 * as an option.
 */
type Catalog = Record<string, { message: string }>;

// Each language, with the wording its `apiKeys.exchangeHelp` used to deny the
// bearer with.
const CATALOGS: { lang: string; catalog: Catalog; denial: RegExp }[] = [
  { lang: "en", catalog: en, denial: /not a bearer/i },
  { lang: "zh", catalog: zh, denial: /不是 ?Bearer/i },
  { lang: "ko", catalog: ko, denial: /bearer 토큰이 아니/i },
  { lang: "ja", catalog: ja, denial: /ベアラートークンではな/ },
  { lang: "es", catalog: es, denial: /no un token de portador/i },
  { lang: "pt", catalog: pt, denial: /não um bearer/i },
  { lang: "ru", catalog: ru, denial: /не bearer/i },
  { lang: "de", catalog: de, denial: /kein Bearer/i },
];

describe("api-keys copy: the key is a bearer token", () => {
  describe.each(CATALOGS)("$lang", ({ catalog, denial }) => {
    it("no message denies that the key is a bearer token", () => {
      const denying = Object.entries(catalog)
        .filter(
          ([, entry]) =>
            /not a bearer/i.test(entry.message) || denial.test(entry.message),
        )
        .map(([key]) => key);
      expect(denying).toEqual([]);
    });

    it("apiKeys.exchangeHelp offers the exchange at the token endpoint", () => {
      const message = catalog["apiKeys.exchangeHelp"]?.message ?? "";
      expect(message).toContain("{tokenEndpoint}");
      expect(message).toContain(
        "grant_type=client_credentials&client_id=…&client_secret=…",
      );
    });

    it("the primary copy names the header the key is sent in", () => {
      // The one instruction a user needs is stated outside the OAuth detail,
      // and the header name is the same in every language.
      expect(catalog["apiKeys.keyHelp"]?.message).toContain(
        "Authorization: Bearer",
      );
    });
  });
});
