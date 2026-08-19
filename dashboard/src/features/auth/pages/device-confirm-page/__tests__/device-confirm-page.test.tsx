import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type {
  DeviceErrorCode,
  DeviceView,
} from "@/common/server-fn/hydra-device";

// The page is a pure render of the loader data the route's GET handler
// produced (mirrors consent-page.test.tsx) — mock the route accessor so the
// test drives that data directly, with no router mount and no Hydra.
const routeData = vi.hoisted(() => ({
  device: null as DeviceView | null,
  deviceErrorCode: null as DeviceErrorCode | null,
  userCode: undefined as string | undefined,
  challenge: undefined as string | undefined,
}));
vi.mock("@tanstack/react-router", async (orig) => ({
  ...(await orig<typeof import("@tanstack/react-router")>()),
  getRouteApi: () => ({
    useLoaderData: () => ({
      device: routeData.device,
      deviceErrorCode: routeData.deviceErrorCode,
    }),
    useSearch: () => ({
      user_code: routeData.userCode,
      device_challenge: routeData.challenge,
    }),
  }),
}));

import DeviceConfirmPage from "@/features/auth/pages/device-confirm-page";

const view = (overrides: Partial<DeviceView> = {}): DeviceView => ({
  userCode: "ABCD-EFGH",
  challenge: "challenge-1",
  ...overrides,
});

const replace = vi.fn();

beforeEach(() => {
  routeData.device = null;
  routeData.deviceErrorCode = null;
  routeData.userCode = undefined;
  routeData.challenge = undefined;
  replace.mockClear();
  vi.stubGlobal("location", {
    href: "https://dashboard.bex.co/auth/device?user_code=ABCD-EFGH&device_challenge=challenge-1",
    replace,
  });
});

describe("DeviceConfirmPage", () => {
  it("shows the code the CLI displayed", () => {
    routeData.device = view();

    render(<DeviceConfirmPage />);

    expect(screen.getByText("ABCD-EFGH")).toBeInTheDocument();
  });

  it("posts the confirmation back with the user code and device challenge", () => {
    routeData.device = view();

    const { container } = render(<DeviceConfirmPage />);

    const form = container.querySelector("form")!;
    expect(form.getAttribute("method")).toBe("POST");
    expect(form.getAttribute("action")).toBe("/auth/device");
    expect(container.querySelector('input[name="user_code"]')).toHaveValue(
      "ABCD-EFGH",
    );
    expect(
      container.querySelector('input[name="device_challenge"]'),
    ).toHaveValue("challenge-1");
  });

  it("reloads as a document when it arrives by client-side navigation with a code + challenge", () => {
    // The login-first bounce returns here via the router, which never runs
    // the GET server handler — so there is no device view to render.
    // Reloading is what makes the handler answer.
    routeData.device = null;
    routeData.userCode = "ABCD-EFGH";
    routeData.challenge = "challenge-1";

    render(<DeviceConfirmPage />);

    expect(replace).toHaveBeenCalledWith(
      "https://dashboard.bex.co/auth/device?user_code=ABCD-EFGH&device_challenge=challenge-1",
    );
  });

  it("shows the expired state — not a reload loop — when there is no code/challenge at all", () => {
    routeData.device = null;
    routeData.userCode = undefined;
    routeData.challenge = undefined;

    render(<DeviceConfirmPage />);

    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByRole("link")).toHaveAttribute("href", "/");
  });

  // w10/m8 t001: a recoverable GET/POST failure renders inside AuthPageShell
  // with error-specific copy instead of the bare text body handleDeviceVerification
  // used to return directly.
  it.each([
    ["missing_code", "Device code expired"],
    ["invalid_code", "Device code expired"],
    ["unconfigured", "Device authorization unavailable"],
    ["unexpected_client", "Authorization refused"],
  ] satisfies [DeviceErrorCode, string][])(
    "renders the %s error state with its own copy",
    (code, expectedTitle) => {
      routeData.device = null;
      routeData.deviceErrorCode = code;
      routeData.userCode = "ABCD-EFGH";
      routeData.challenge = "challenge-1";

      render(<DeviceConfirmPage />);

      expect(screen.getByText(expectedTitle)).toBeInTheDocument();
    },
  );

  it("never reloads a recovered error code — the handler already answered", () => {
    routeData.device = null;
    routeData.deviceErrorCode = "invalid_code";
    routeData.userCode = "ABCD-EFGH";
    routeData.challenge = "challenge-1";

    render(<DeviceConfirmPage />);

    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByRole("link")).toHaveAttribute("href", "/");
  });
});
