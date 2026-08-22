import { filterItems } from "../filter";

const items = [
  { label: "bex-co/web-onefx-boilerplate", value: "a" },
  { label: "bex-co/onefx", value: "b" },
  { label: "bex-co/mobile-galiao", value: "c" },
];

describe("searchable list filter", () => {
  it("keeps every item for a blank query, sorted by repo name", () => {
    // Sorted by the name after "/": mobile-galiao, onefx, web-onefx-boilerplate.
    expect(filterItems(items, "  ").map((i) => i.value)).toEqual([
      "c",
      "b",
      "a",
    ]);
  });

  it("matches a case-insensitive substring and sorts results by repo name", () => {
    // Both names contain "onefx"; "onefx" sorts before "web-onefx-boilerplate".
    expect(filterItems(items, "  ONEFX ").map((i) => i.value)).toEqual([
      "b",
      "a",
    ]);
    expect(filterItems(items, "galiao").map((i) => i.value)).toEqual(["c"]);
  });

  it("ranks repo-name matches above owner-only matches", () => {
    const mixed = [
      { label: "onefx-labs/tools", value: "owner" }, // matches via owner
      { label: "bex-co/onefx", value: "name" }, // matches via repo name
    ];
    expect(filterItems(mixed, "onefx").map((i) => i.value)).toEqual([
      "name",
      "owner",
    ]);
  });

  it("returns an empty list when nothing matches", () => {
    expect(filterItems(items, "nope")).toEqual([]);
  });
});
