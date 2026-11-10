import { describe, expect, it } from "vitest";
import { validateWebhookForm } from "../validation";

const messages = {
  nameRequired: "required",
  nameInvalid: "invalid",
  nameDuplicate: "duplicate",
  urlRequired: "url required",
  urlHTTPS: "https",
  eventsRequired: "events",
};
const base = {
  name: "alerts",
  url: "https://example.com/hook",
  selectedEventCount: 1,
  existingNames: [] as string[],
};

// w21/036: the webhook name is held to the server's label rule before submit.
describe("validateWebhookForm name", () => {
  it("refuses 101 characters and control characters, accepts 100 CJK", () => {
    expect(
      validateWebhookForm({ ...base, name: "a".repeat(101) }, messages).name,
    ).toBe("invalid");
    expect(validateWebhookForm({ ...base, name: "a\nb" }, messages).name).toBe(
      "invalid",
    );
    expect(
      validateWebhookForm({ ...base, name: "受".repeat(100) }, messages).name,
    ).toBeUndefined();
  });

  it("keeps the required and duplicate rules", () => {
    expect(validateWebhookForm({ ...base, name: "  " }, messages).name).toBe(
      "required",
    );
    expect(
      validateWebhookForm({ ...base, existingNames: ["Alerts"] }, messages)
        .name,
    ).toBe("duplicate");
  });
});
