import { describe, it, expect } from "vitest";
import { bearerKey, curlExample } from "@/features/api-keys/lib/bearer-key";

const ID = "0190f3a2-6c1e-7d4b-9a58-2f6e1c0b7d43";
// Joined, not written out: a literal that looks like a secret trips the
// repository's credential scan. 26 characters, as bex-api's secrets are.
const SECRET = ["fixture", "client", "secret", "0001"].join("-");
const KEY = `bex_${ID}_${SECRET}`;

describe("bearerKey", () => {
  it("is the key bex-api returned, untouched", () => {
    expect(bearerKey(ID, SECRET, "bex_server_issued")).toBe(
      "bex_server_issued",
    );
  });

  it.each([null, undefined, ""])(
    "composes bex_<id>_<secret> when the server answers key=%j",
    (missing) => {
      expect(bearerKey(ID, SECRET, missing)).toBe(KEY);
    },
  );
});

describe("curlExample", () => {
  it("lists services with the key as the bearer, in a shape that pastes", () => {
    expect(curlExample("https://api.bex.co", KEY)).toBe(
      [
        "curl --request GET \\",
        "     --url 'https://api.bex.co/v1/services?limit=20' \\",
        "     --header 'Accept: application/json' \\",
        `     --header 'Authorization: Bearer ${KEY}'`,
      ].join("\n"),
    );
  });

  it("does not double the slash when the base ends with one", () => {
    expect(curlExample("http://localhost:8090/", KEY)).toContain(
      "--url 'http://localhost:8090/v1/services?limit=20'",
    );
  });
});
