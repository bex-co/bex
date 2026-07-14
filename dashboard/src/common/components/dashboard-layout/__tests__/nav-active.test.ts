import { describe, it, expect } from "vitest";
import { isNavItemActive } from "../nav-active";

describe("isNavItemActive", () => {
  describe('the "Projects" item (to: "/")', () => {
    it.each([
      "/",
      "/project/prj-1",
      "/services/eden-cms-v2",
      "/services/new",
      "/databases/orders-db",
      "/keyvalue/sessions-cache",
      "/keyvalue/new",
    ])("is active on %s", (pathname) => {
      expect(isNavItemActive(pathname, "/")).toBe(true);
    });

    it.each(["/usage", "/settings"])("is not active on %s", (pathname) => {
      expect(isNavItemActive(pathname, "/")).toBe(false);
    });
  });

  describe("other items", () => {
    it("matches the exact path", () => {
      expect(isNavItemActive("/usage", "/usage")).toBe(true);
    });

    it("matches a sub-path", () => {
      expect(isNavItemActive("/settings/team", "/settings")).toBe(true);
    });

    it("doesn't match an unrelated path", () => {
      expect(isNavItemActive("/", "/usage")).toBe(false);
      expect(isNavItemActive("/usagex", "/usage")).toBe(false);
    });
  });
});
