import appConfig from "../../app.json";

describe("mobile configuration", () => {
  it("uses only bex-owned identifiers and no dangerous permissions", () => {
    expect(appConfig.expo.name).toBe("bex");
    expect(appConfig.expo.scheme).toEqual(["bex", "co.bex.mobile"]);
    expect(appConfig.expo.ios.bundleIdentifier).toBe("co.bex.mobile");
    expect(appConfig.expo.android.package).toBe("co.bex.mobile");
    expect(appConfig.expo.android.permissions).toEqual([]);
    expect(JSON.stringify(appConfig)).not.toContain("projectId");
    expect(JSON.stringify(appConfig)).not.toContain("updates");
  });

  it("claims only the verified production invite URL", () => {
    expect(appConfig.expo.ios.associatedDomains).toEqual([
      "applinks:dashboard.bex.co",
    ]);
    expect(appConfig.expo.android.intentFilters).toEqual([
      {
        action: "VIEW",
        autoVerify: true,
        data: [
          {
            scheme: "https",
            host: "dashboard.bex.co",
            path: "/invite",
          },
        ],
        category: ["BROWSABLE", "DEFAULT"],
      },
    ]);

    const association = JSON.stringify({
      ios: appConfig.expo.ios.associatedDomains,
      android: appConfig.expo.android.intentFilters,
    });
    expect(association).not.toContain("http:");
    expect(association).not.toContain("bex:");
    expect(association).not.toContain("pathPrefix");
    expect(association).not.toContain("*");
  });
});
