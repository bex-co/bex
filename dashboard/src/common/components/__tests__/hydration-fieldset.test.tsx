import { describe, expect, it } from "vitest";
import { act } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { hydrateRoot } from "react-dom/client";
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
    const container = document.createElement("div");
    container.innerHTML = renderToString(form);
    document.body.appendChild(container);
    const input = () => container.querySelector("input")!;
    expect(container.querySelector("fieldset")!.disabled).toBe(true);
    expect(input().matches(":disabled")).toBe(true);

    let root!: ReturnType<typeof hydrateRoot>;
    await act(async () => {
      root = hydrateRoot(container, form);
    });
    expect(container.querySelector("fieldset")!.disabled).toBe(false);
    expect(container.querySelector("fieldset")!.hasAttribute("aria-busy")).toBe(
      false,
    );
    expect(input().matches(":disabled")).toBe(false);
    act(() => root.unmount());
    container.remove();
  });
});
