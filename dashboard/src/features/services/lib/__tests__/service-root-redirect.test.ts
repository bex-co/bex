import { describe, expect, it } from "vitest";
import { isRedirect } from "@tanstack/react-router";
import { redirectServiceRoot } from "../service-root-redirect";

describe("redirectServiceRoot", () => {
  it("lands the service root on Deploys", () => {
    try {
      redirectServiceRoot("srv-web");
      throw new Error("expected a redirect");
    } catch (error) {
      expect(isRedirect(error)).toBe(true);
      expect(error).toMatchObject({
        options: {
          to: "/services/$serviceId/deploys",
          params: { serviceId: "srv-web" },
          replace: true,
        },
      });
    }
  });
});
