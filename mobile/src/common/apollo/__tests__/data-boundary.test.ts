import { DataBoundary } from "../data-boundary";

describe("DataBoundary", () => {
  it("aborts and invalidates in-flight work before a workspace switch", async () => {
    const boundary = new DataBoundary();
    const oldRequest = boundary.begin();
    expect(oldRequest.isCurrent()).toBe(true);

    await boundary.reset("tea-second");

    expect(oldRequest.signal.aborted).toBe(true);
    expect(oldRequest.isCurrent()).toBe(false);
    expect(boundary.workspaceId).toBe("tea-second");
    expect(boundary.begin().isCurrent()).toBe(true);
  });

  it("awaits cache clearing before exposing the new boundary", async () => {
    const boundary = new DataBoundary();
    const calls: string[] = [];
    boundary.registerResetHandler(async () => {
      await Promise.resolve();
      calls.push("cleared");
    });

    await boundary.reset(null);

    expect(calls).toEqual(["cleared"]);
  });

  it("initializes the first workspace without invalidating caller bootstrap", () => {
    const boundary = new DataBoundary();
    const bootstrap = boundary.begin();
    boundary.initialize("tea-first");
    expect(bootstrap.isCurrent()).toBe(true);
    expect(boundary.workspaceId).toBe("tea-first");
  });
});
