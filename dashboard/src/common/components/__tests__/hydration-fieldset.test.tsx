import { describe, expect, it } from "vitest";
import { serverRendered } from "@/test/hydration";
import { HydrationFieldset } from "../hydration-fieldset";

// w4/m219 (a797/a800/a815): a server-rendered create form or search box took
// typing before React attached its handlers, then hydration re-applied the
// controlled value — the draft name vanished and a Port edit snapped back to
// 3000. The fieldset makes the server HTML itself uneditable, and hydration
// enables it in place.
describe("HydrationFieldset", () => {
  const form = (
    <HydrationFieldset>
      <input aria-label="Name" defaultValue="" />
    </HydrationFieldset>
  );

  it("ships disabled server HTML and enables it on hydration", async () => {
    const { container, hydrate } = serverRendered(form);
    const fieldset = () => container.querySelector("fieldset")!;
    const input = () => container.querySelector("input")!;
    expect(fieldset().disabled).toBe(true);
    expect(input().matches(":disabled")).toBe(true);

    await hydrate();
    expect(fieldset().disabled).toBe(false);
    expect(fieldset().hasAttribute("aria-busy")).toBe(false);
    expect(input().matches(":disabled")).toBe(false);
  });
});
