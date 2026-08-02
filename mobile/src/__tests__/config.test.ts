import appConfig from "../../app.json";

describe("mobile configuration", () => {
  it("uses only bex-owned identifiers and no dangerous permissions", () => {
    expect(appConfig.expo.name).toBe("bex");
    expect(appConfig.expo.scheme).toBe("bex");
    expect(appConfig.expo.ios.bundleIdentifier).toBe("co.bex.mobile");
    expect(appConfig.expo.android.package).toBe("co.bex.mobile");
    expect(appConfig.expo.android.permissions).toEqual([]);
    expect(JSON.stringify(appConfig)).not.toContain("projectId");
    expect(JSON.stringify(appConfig)).not.toContain("updates");
  });
});
