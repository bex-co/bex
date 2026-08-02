import {
  bootstrapInviteLink,
  verifiedInviteToken,
} from "../invite-link-bootstrap";
import { parseInviteToken } from "../invite-token";

const token = "0123456789abcdef0123456789abcdef";

describe("invite link bootstrap", () => {
  it("accepts only the exact verified URL matching the router parameter", () => {
    expect(
      verifiedInviteToken(
        `https://dashboard.bex.co/invite?invite=${token}`,
        token,
      ),
    ).toBe(token);

    for (const [url, parameter] of [
      [`http://dashboard.bex.co/invite?invite=${token}`, token],
      [`bex://invite?invite=${token}`, token],
      [`co.bex.mobile://invite?invite=${token}`, token],
      [`https://dashboard.bex.co.evil/invite?invite=${token}`, token],
      [`https://dashboard.bex.co./invite?invite=${token}`, token],
      [`https://dashboard.bex.co:444/invite?invite=${token}`, token],
      [`https://user@dashboard.bex.co/invite?invite=${token}`, token],
      [`https://dashboard.bex.co/invite/?invite=${token}`, token],
      [`https://dashboard.bex.co/other?invite=${token}`, token],
      [`https://dashboard.bex.co/invite?invite=${token}&utm=x`, token],
      [
        `https://dashboard.bex.co/invite?invite=${token}&invite=${token}`,
        token,
      ],
      [`https://dashboard.bex.co/invite?invite=${token}#fragment`, token],
      [`https://dashboard.bex.co/invite?invite=${token}#`, token],
      [
        `https://dashboard.bex.co/invite?invite=${token}`,
        "abcdef0123456789abcdef0123456789",
      ],
      [`https://dashboard.bex.co/invite?invite=${token}`, [token]],
      [null, token],
    ] as const) {
      expect(verifiedInviteToken(url, parameter)).toBe(null);
    }
  });

  it("scrubs the visible route before unresolved secure storage", async () => {
    const calls: string[] = [];
    let resolve = (_accepted: boolean) => {};
    const pending = bootstrapInviteLink(
      "0123456789abcdef0123456789abcdef",
      () => calls.push("scrub"),
      async () => {
        calls.push("capture");
        return new Promise<boolean>((done) => {
          resolve = done;
        });
      },
    );

    expect(calls).toEqual(["scrub", "capture"]);
    resolve(true);
    expect(await pending).toBe(true);
  });

  it("scrubs but refuses malformed and unverified values", async () => {
    for (const value of [
      [token],
      token.toUpperCase(),
      `https://evil.example/invite?invite=${token}`,
      `bex://invite?invite=${token}`,
    ]) {
      const calls: string[] = [];
      const accepted = await bootstrapInviteLink(
        value,
        () => calls.push("scrub"),
        async (candidate) => {
          calls.push("capture");
          return parseInviteToken(candidate) !== null;
        },
      );
      expect(calls).toEqual(["scrub", "capture"]);
      expect(accepted).toBe(false);
    }
  });

  it("scrubs before a capture/storage rejection escapes", async () => {
    const calls: string[] = [];
    let failed = false;
    try {
      await bootstrapInviteLink(
        token,
        () => calls.push("scrub"),
        async () => {
          calls.push("capture");
          throw new Error("storage unavailable");
        },
      );
    } catch {
      failed = true;
    }
    expect(failed).toBe(true);
    expect(calls).toEqual(["scrub", "capture"]);
  });
});
