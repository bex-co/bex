import { parseInviteToken, parseStoredInvite } from "../invite-token";

const token = "0123456789abcdef0123456789abcdef";

describe("invite bearer validation", () => {
  it("accepts only one exact lowercase 128-bit hex token", () => {
    expect(parseInviteToken(token)).toBe(token);
    for (const value of [
      token.toUpperCase(),
      token.slice(1),
      `${token}0`,
      ` ${token}`,
      [token],
      null,
      undefined,
    ]) {
      expect(parseInviteToken(value)).toBe(null);
    }
  });

  it("rejects malformed, extended, and unbound stored envelopes", () => {
    expect(parseStoredInvite({ version: 1, token, subject: null })).toEqual({
      version: 1,
      token,
      subject: null,
    });
    expect(
      parseStoredInvite({ version: 1, token, subject: "identity-a" }),
    ).toEqual({ version: 1, token, subject: "identity-a" });
    expect(
      parseStoredInvite({ version: 1, token, subject: null, extra: true }),
    ).toBe(null);
    expect(parseStoredInvite({ version: 2, token, subject: null })).toBe(null);
    expect(parseStoredInvite({ version: 1, token, subject: "" })).toBe(null);
  });
});
