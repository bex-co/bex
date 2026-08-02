import { bootstrapInviteLink } from "../invite-link-bootstrap";

describe("invite link bootstrap", () => {
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
});
