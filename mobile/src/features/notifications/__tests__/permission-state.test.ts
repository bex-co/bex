import { notificationPermissionState } from "../permission-state";

describe("notificationPermissionState", () => {
  it("treats iOS authorized, provisional, and ephemeral states as granted", () => {
    for (const status of [2, 3, 4]) {
      expect(
        notificationPermissionState(
          { status: "undetermined", ios: { status } },
          "ios",
        ),
      ).toBe("granted");
    }
  });

  it("uses iOS denied and not-determined states instead of the root status", () => {
    expect(
      notificationPermissionState(
        { status: "granted", ios: { status: 1 } },
        "ios",
      ),
    ).toBe("denied");
    expect(
      notificationPermissionState(
        { status: "granted", ios: { status: 0 } },
        "ios",
      ),
    ).toBe("undetermined");
  });

  it("keeps Android on the root Expo permission status", () => {
    expect(
      notificationPermissionState(
        { status: "denied", ios: { status: 2 } },
        "android",
      ),
    ).toBe("denied");
    expect(notificationPermissionState({ status: "granted" }, "android")).toBe(
      "granted",
    );
  });
});
