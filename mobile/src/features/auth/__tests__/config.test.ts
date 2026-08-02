import {
  MOBILE_CLIENT_ID,
  MOBILE_REDIRECT_URI,
  readMobileConfig,
} from "../config";

describe("mobile auth configuration", () => {
  it("defaults to the fixed first-party public client", () => {
    const config = readMobileConfig({}, false);
    expect(config.oauthClientId).toBe(MOBILE_CLIENT_ID);
    expect(config.oauthRedirectUri).toBe(MOBILE_REDIRECT_URI);
    expect(config.graphqlUrl).toBe("https://api.bex.co/graphql");
  });

  it("allows plain HTTP only for local development", () => {
    const env = {
      EXPO_PUBLIC_BEX_API_URL: "http://127.0.0.1:8090",
      EXPO_PUBLIC_BEX_OAUTH_ISSUER: "http://localhost:4444",
    };
    expect(readMobileConfig(env, true).apiOrigin).toBe("http://127.0.0.1:8090");
    expect(() => readMobileConfig(env, false)).toThrow(/HTTPS/);
  });

  it("rejects ambient URL credentials and redirect-like suffixes", () => {
    for (const value of [
      "https://user:secret@api.bex.co",
      "https://api.bex.co?next=https://attacker.test",
      "https://api.bex.co#attacker",
    ]) {
      expect(() =>
        readMobileConfig({ EXPO_PUBLIC_BEX_API_URL: value }, false),
      ).toThrow();
    }
  });
});
