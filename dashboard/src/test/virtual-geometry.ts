import { afterEach, beforeEach } from "vitest";

// Scoped jsdom layout-geometry harness for @tanstack/react-virtual tests.
//
// jsdom performs no layout: every element reports `offsetWidth`/`offsetHeight`
// of 0. @tanstack/react-virtual measures both the scroll viewport and each row
// through `offsetHeight` (virtual-core's `getRect` and default `measureElement`),
// so a virtualizer under jsdom sees a 0-height viewport, computes an empty
// visible range, and renders nothing — which is exactly why the w9/m63
// virtualization attempt was reverted as unverifiable.
//
// This helper synthesizes just enough geometry for the virtualizer to compute a
// real visible window: the scroll viewport (marked `data-log-viewport`) gets a
// fixed `offsetHeight`, and each measured row (marked `data-index`) gets a fixed
// row `offsetHeight`. It is deliberately SCOPED — it redefines the
// `HTMLElement.prototype` offset getters only between the `beforeEach`/
// `afterEach` of the file that calls `setupVirtualGeometry()`, and restores the
// original descriptors afterward — so it can never perturb the other ~2,100
// dashboard tests the way a global stub in `src/test/setup.ts` would.

const VIEWPORT_HEIGHT = 520;
const VIEWPORT_WIDTH = 800;
const ROW_HEIGHT = 24;

/** Row height the harness reports — matches the component's `estimateSize`. */
export const VIRTUAL_ROW_HEIGHT = ROW_HEIGHT;
/** Viewport height the harness reports. */
export const VIRTUAL_VIEWPORT_HEIGHT = VIEWPORT_HEIGHT;

/**
 * Roughly how many rows fit the synthetic viewport (used to bound assertions on
 * the rendered window — a virtualized list should render on this order, not the
 * whole buffer).
 */
export const VIRTUAL_VISIBLE_ROWS = Math.ceil(VIEWPORT_HEIGHT / ROW_HEIGHT);

function syntheticHeight(el: HTMLElement): number {
  if (el.hasAttribute("data-log-viewport")) return VIEWPORT_HEIGHT;
  if (el.hasAttribute("data-index")) return ROW_HEIGHT;
  return 0;
}

function syntheticWidth(el: HTMLElement): number {
  if (el.hasAttribute("data-log-viewport") || el.hasAttribute("data-index")) {
    return VIEWPORT_WIDTH;
  }
  return 0;
}

/**
 * Install the synthetic geometry for the current test file. Call once at the top
 * of a `describe` block; it wires its own `beforeEach`/`afterEach`.
 */
export function setupVirtualGeometry(): void {
  let originalHeight: PropertyDescriptor | undefined;
  let originalWidth: PropertyDescriptor | undefined;

  beforeEach(() => {
    originalHeight = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype,
      "offsetHeight",
    );
    originalWidth = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype,
      "offsetWidth",
    );
    Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
      configurable: true,
      get(this: HTMLElement) {
        return syntheticHeight(this);
      },
    });
    Object.defineProperty(HTMLElement.prototype, "offsetWidth", {
      configurable: true,
      get(this: HTMLElement) {
        return syntheticWidth(this);
      },
    });
  });

  afterEach(() => {
    restore("offsetHeight", originalHeight);
    restore("offsetWidth", originalWidth);
  });
}

function restore(prop: string, descriptor: PropertyDescriptor | undefined) {
  if (descriptor) {
    Object.defineProperty(HTMLElement.prototype, prop, descriptor);
  } else {
    delete (HTMLElement.prototype as unknown as Record<string, unknown>)[prop];
  }
}

/**
 * Drive a scroll event on the viewport with an explicit scroll geometry — for
 * the pin/follow tests, which assert the "distance from bottom" math rather than
 * the virtualized window. Sets the three scroll properties on the specific node
 * (not the prototype) and dispatches a native `scroll` event.
 */
export function scrollViewport(
  viewport: HTMLElement,
  { scrollTop, scrollHeight, clientHeight }: ScrollGeometry,
): void {
  defineScrollProp(viewport, "scrollHeight", scrollHeight);
  defineScrollProp(viewport, "clientHeight", clientHeight);
  viewport.scrollTop = scrollTop;
  viewport.dispatchEvent(new Event("scroll"));
}

interface ScrollGeometry {
  scrollTop: number;
  scrollHeight: number;
  clientHeight: number;
}

function defineScrollProp(el: HTMLElement, prop: string, value: number): void {
  Object.defineProperty(el, prop, {
    configurable: true,
    value,
  });
}
