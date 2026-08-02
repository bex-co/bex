import {
  confirmEnvironmentEditIntent,
  createEnvironmentEditIntent,
  EnvironmentSecretSession,
} from "../environment-intent";

describe("EnvironmentSecretSession", () => {
  it("holds only one revealed key and clears it without persistence", () => {
    const session = new EnvironmentSecretSession();
    session.reveal({
      serviceId: "srv-one",
      key: "FIRST",
      value: "first-secret",
      revision: "evr1_first",
    });
    session.reveal({
      serviceId: "srv-one",
      key: "SECOND",
      value: "second-secret",
      revision: "evr1_second",
    });
    expect(session.value()).toEqual({
      serviceId: "srv-one",
      key: "SECOND",
      value: "second-secret",
      revision: "evr1_second",
    });
    session.clear();
    expect(session.value()).toBe(null);
  });

  it("notifies boundary consumers when a value is edited and cleared", () => {
    const session = new EnvironmentSecretSession();
    const observed: Array<string | null> = [];
    session.subscribe((value) => observed.push(value?.value ?? null));
    session.reveal({
      serviceId: "srv-one",
      key: "TOKEN",
      value: "old",
      revision: "evr1_one",
    });
    session.edit("new");
    session.clear();
    expect(observed).toEqual(["old", "new", null]);
  });
});

describe("environment edit intent", () => {
  it("freezes the exact service, key, value, and revision confirmed by the user", () => {
    const draft = {
      serviceId: "srv-one",
      key: "TOKEN",
      value: "secret-never-rendered",
      revision: "evr1_one",
    };
    const requested = createEnvironmentEditIntent(draft, "api");
    draft.value = "changed-after-request";
    const confirmed = confirmEnvironmentEditIntent(requested);

    expect(Object.isFrozen(requested)).toBe(true);
    expect(Object.isFrozen(confirmed)).toBe(true);
    expect(confirmed.action.confirmed).toBe(true);
    expect(confirmed.serviceId).toBe("srv-one");
    expect(confirmed.key).toBe("TOKEN");
    expect(confirmed.value).toBe("secret-never-rendered");
    expect(confirmed.revision).toBe("evr1_one");
    expect(confirmed.action.target.label).toBe("api · TOKEN");
    expect(confirmed.action.target.label.includes(confirmed.value)).toBe(false);
  });
});
