import { parseMaskedEnvironmentList } from "../environment-masked-list";

describe("parseMaskedEnvironmentList", () => {
  it("accepts an empty list and one coherent values-free revision snapshot", () => {
    expect(parseMaskedEnvironmentList([])).toEqual({
      valid: true,
      variables: [],
    });
    expect(
      parseMaskedEnvironmentList([
        { id: "A", key: "A", revision: "evr1_same" },
        { id: "B", key: "B", revision: "evr1_same" },
      ]),
    ).toEqual({
      valid: true,
      variables: [
        { id: "A", key: "A", revision: "evr1_same" },
        { id: "B", key: "B", revision: "evr1_same" },
      ],
    });
  });

  it("fails closed on missing, mixed, or duplicate revisions", () => {
    for (const items of [
      [{ id: "A", key: "A", revision: null }],
      [
        { id: "A", key: "A", revision: "evr1_one" },
        { id: "B", key: "B", revision: "evr1_two" },
      ],
      [
        { id: "A", key: "A", revision: "evr1_one" },
        { id: "A2", key: "A", revision: "evr1_one" },
      ],
      [null],
    ]) {
      expect(parseMaskedEnvironmentList(items)).toEqual({
        valid: false,
        variables: [],
      });
    }
  });
});
