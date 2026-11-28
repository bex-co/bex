import { describe, expect, it } from "vitest";
import {
  ALLOW_ALL_ENTRIES,
  ALLOW_ALL_PAIR_ENTRIES,
  checkInboundAddress,
  inboundRangesAllowedByEveryLevel,
  type IPAllowListEntryDraft,
} from "@/common/lib/ip-allow-list";

const rules = (...cidrs: string[]): IPAllowListEntryDraft[] =>
  cidrs.map((cidrBlock) => ({ cidrBlock, description: "" }));

// The milestone's worked example (w4/m710 t003): only 10.1.2.0/24 passes all
// three levels.
const workspace = rules("10.0.0.0/8");
const environment = rules("10.1.0.0/16");
const service = rules("10.1.2.0/24", "192.168.0.0/16");

describe("inboundRangesAllowedByEveryLevel", () => {
  it("keeps the narrower of each nested pair across levels", () => {
    expect(
      inboundRangesAllowedByEveryLevel([workspace, environment, service]),
    ).toEqual(["10.1.2.0/24"]);
  });

  it("drops ranges a level does not overlap at all", () => {
    expect(
      inboundRangesAllowedByEveryLevel([
        rules("10.0.0.0/8"),
        rules("192.168.0.0/16", "172.16.0.0/12"),
      ]),
    ).toEqual([]);
  });

  it("skips a level that does not apply, but an empty level allows nothing", () => {
    expect(
      inboundRangesAllowedByEveryLevel([workspace, undefined, service]),
    ).toEqual(["10.1.2.0/24"]);
    expect(inboundRangesAllowedByEveryLevel([workspace, [], service])).toEqual(
      [],
    );
    expect(inboundRangesAllowedByEveryLevel([undefined])).toEqual([]);
  });

  it("counts Render's defaults as one IPv4 range: an IPv6 range is skipped, as Render's view skips it", () => {
    expect(
      inboundRangesAllowedByEveryLevel([
        ALLOW_ALL_PAIR_ENTRIES,
        ALLOW_ALL_PAIR_ENTRIES,
        ALLOW_ALL_ENTRIES,
      ]),
    ).toEqual(["0.0.0.0/0"]);
    expect(
      inboundRangesAllowedByEveryLevel([
        ALLOW_ALL_PAIR_ENTRIES,
        ALLOW_ALL_PAIR_ENTRIES,
      ]),
    ).toEqual(["0.0.0.0/0"]);
    expect(
      inboundRangesAllowedByEveryLevel([rules("::/0"), rules("2001:db8::/32")]),
    ).toEqual([]);
  });

  it("matches a rule written with host bits by its masked range, and counts duplicates once", () => {
    expect(
      inboundRangesAllowedByEveryLevel([
        rules("10.1.2.3/16"),
        rules("10.1.0.0/16", "10.1.9.9/16"),
      ]),
    ).toEqual(["10.1.0.0/16"]);
  });

  it("ignores a draft row that is not a CIDR yet", () => {
    expect(
      inboundRangesAllowedByEveryLevel([
        rules("0.0.0.0/0"),
        rules("10.0.0", "10.0.0.0/33", "300.0.0.0/8", "10.0.0.0/8"),
      ]),
    ).toEqual(["10.0.0.0/8"]);
  });

  it("keeps /32 and /0 edges exact", () => {
    expect(
      inboundRangesAllowedByEveryLevel([
        rules("0.0.0.0/0"),
        rules("255.255.255.255/32"),
      ]),
    ).toEqual(["255.255.255.255/32"]);
  });
});

describe("checkInboundAddress", () => {
  const levels = { workspace, environment, own: service };

  it("says nothing before an address is typed, and refuses what is not an IPv4 address", () => {
    expect(checkInboundAddress("", levels)).toEqual({ state: "empty" });
    expect(checkInboundAddress("   ", levels)).toEqual({ state: "empty" });
    for (const text of [
      "10.1.2",
      "10.1.2.256",
      "10.01.2.3",
      "2001:db8::1",
      "x",
    ]) {
      expect(checkInboundAddress(text, levels)).toEqual({ state: "invalid" });
    }
  });

  it("allows an address every level admits", () => {
    expect(checkInboundAddress("10.1.2.3", levels)).toEqual({
      state: "allowed",
    });
    expect(checkInboundAddress(" 10.1.2.3 ", levels)).toEqual({
      state: "allowed",
    });
  });

  it("names the first level that refuses: workspace, then environment, then the resource's own", () => {
    expect(checkInboundAddress("192.168.1.1", levels)).toEqual({
      state: "refused",
      level: "workspace",
    });
    expect(checkInboundAddress("10.2.0.1", levels)).toEqual({
      state: "refused",
      level: "environment",
    });
    expect(checkInboundAddress("10.1.3.1", levels)).toEqual({
      state: "refused",
      level: "own",
    });
  });

  it("checks no environment level outside an environment", () => {
    expect(
      checkInboundAddress("10.2.0.1", { workspace, own: rules("10.0.0.0/8") }),
    ).toEqual({ state: "allowed" });
  });

  it("refuses everything at a level with no rule, or with only IPv6 rules", () => {
    expect(
      checkInboundAddress("10.1.2.3", { ...levels, workspace: [] }),
    ).toEqual({ state: "refused", level: "workspace" });
    expect(
      checkInboundAddress("10.1.2.3", {
        ...levels,
        environment: rules("::/0"),
      }),
    ).toEqual({ state: "refused", level: "environment" });
    expect(checkInboundAddress("10.1.2.3", { ...levels, own: [] })).toEqual({
      state: "refused",
      level: "own",
    });
  });
});
