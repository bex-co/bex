import { LogoutController } from "../logout-controller";

describe("LogoutController", () => {
  it("does nothing when the confirmation is canceled", async () => {
    let signOuts = 0;
    const controller = new LogoutController({
      confirm: async () => false,
      signOut: async () => {
        signOuts += 1;
      },
    });
    expect(await controller.request()).toBe("canceled");
    expect(signOuts).toBe(0);
  });

  it("runs exactly one signOut when confirmed and reports pending phases", async () => {
    let signOuts = 0;
    const pending: boolean[] = [];
    const controller = new LogoutController({
      confirm: async () => true,
      signOut: async () => {
        signOuts += 1;
      },
      onPending: (value) => pending.push(value),
    });
    expect(await controller.request()).toBe("done");
    expect(signOuts).toBe(1);
    expect(pending).toEqual([true, false]);
  });

  it("refuses a second request while one is in flight (double-tap guard)", async () => {
    let confirms = 0;
    let releaseConfirm!: (value: boolean) => void;
    const controller = new LogoutController({
      confirm: () => {
        confirms += 1;
        return new Promise<boolean>((resolve) => {
          releaseConfirm = resolve;
        });
      },
      signOut: async () => undefined,
    });
    const first = controller.request();
    const second = await controller.request();
    expect(second).toBe("skipped");
    releaseConfirm(false);
    expect(await first).toBe("canceled");
    // Only the first tap ever opened a confirmation dialog.
    expect(confirms).toBe(1);
    expect(controller.isBusy()).toBe(false);
  });

  it("reports a failed local teardown distinctly and clears busy", async () => {
    const controller = new LogoutController({
      confirm: async () => true,
      signOut: async () => {
        throw new Error("storage clear failed");
      },
    });
    expect(await controller.request()).toBe("failed");
    expect(controller.isBusy()).toBe(false);
    // A later attempt is allowed once the failed one settled.
    expect(await controller.request()).toBe("failed");
  });
});
