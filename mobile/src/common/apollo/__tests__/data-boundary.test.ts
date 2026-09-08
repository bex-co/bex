import { DataBoundary } from "../data-boundary";

describe("DataBoundary", () => {
  it("publishes invalidation before async cleanup and supports unsubscribing", async () => {
    const boundary = new DataBoundary();
    const request = boundary.begin();
    const observed: number[] = [];
    let finishCleanup: () => void = () => undefined;
    boundary.registerResetHandler(
      () =>
        new Promise<void>((resolve) => {
          finishCleanup = resolve;
        }),
    );
    const unsubscribe = boundary.subscribe(() => {
      expect(request.signal.aborted).toBe(true);
      observed.push(boundary.getGeneration());
    });
    const reset = boundary.reset("tea-next");
    expect(observed).toEqual([1]);
    finishCleanup();
    await reset;
    unsubscribe();
    const second = boundary.reset(null);
    finishCleanup();
    await second;
    expect(observed).toEqual([1]);
    expect(boundary.getGeneration()).toBe(2);
  });
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
