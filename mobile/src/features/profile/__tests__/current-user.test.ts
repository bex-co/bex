import {
  identityDisplay,
  initialsFor,
  parseCurrentUser,
} from "../current-user";

describe("parseCurrentUser", () => {
  it("validates and trims a full response", () => {
    expect(
      parseCurrentUser({ name: "  Ada Lovelace ", email: " a@x.io " }),
    ).toEqual({ name: "Ada Lovelace", email: "a@x.io" });
  });

  it("normalizes missing or blank members to empty strings, never guesses", () => {
    expect(parseCurrentUser({ email: "only@x.io" })).toEqual({
      name: "",
      email: "only@x.io",
    });
    expect(parseCurrentUser({ name: "", email: "" })).toEqual({
      name: "",
      email: "",
    });
  });

  it("rejects a non-object body as a protocol error", () => {
    expect(() => parseCurrentUser(null)).toThrow();
    expect(() => parseCurrentUser("nope")).toThrow();
    expect(() => parseCurrentUser([{ name: "x" }])).toThrow();
  });
});

describe("identityDisplay fallbacks", () => {
  it("shows the name primary with the email beneath", () => {
    expect(
      identityDisplay({ name: "Ada Lovelace", email: "a@x.io" }, "Signed in"),
    ).toEqual({
      primary: "Ada Lovelace",
      secondary: "a@x.io",
      initials: "AL",
      identified: true,
    });
  });

  it("promotes an email-only response to the primary line", () => {
    expect(
      identityDisplay({ name: "", email: "only@x.io" }, "Signed in"),
    ).toEqual({
      primary: "only@x.io",
      secondary: null,
      initials: "O",
      identified: true,
    });
  });

  it("uses the honest signed-in fallback for an empty identity", () => {
    expect(identityDisplay({ name: "", email: "" }, "Signed in")).toEqual({
      primary: "Signed in",
      secondary: null,
      initials: "",
      identified: false,
    });
    expect(identityDisplay(null, "Signed in").identified).toBe(false);
  });
});

describe("initialsFor", () => {
  it("takes up to two name initials", () => {
    expect(initialsFor("Grace Brewster Hopper", "")).toBe("GB");
    expect(initialsFor("cher", "")).toBe("C");
  });
  it("falls back to the email's first alphanumeric, then to nothing", () => {
    expect(initialsFor("", "9lives@x.io")).toBe("9");
    expect(initialsFor("", "")).toBe("");
    // A subject-like opaque value yields no fragment of the id as an initial.
    expect(initialsFor("", "!!!@@@")).toBe("");
  });
});
