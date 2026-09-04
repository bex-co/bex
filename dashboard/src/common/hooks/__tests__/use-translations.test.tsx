import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import i18n from "@/i18n/init";
import { useTranslations } from "../use-translations";

describe("useTranslations", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("returns the translated message for a known key", () => {
    const { result } = renderHook(() => useTranslations());

    expect(result.current.t("common.appName")).toBe("bex");
  });

  it("interpolates params into the message", () => {
    const { result } = renderHook(() => useTranslations());

    expect(
      result.current.t("metrics.requestsCount", {
        count: 7266,
        formatted: "7,266",
      }),
    ).toBe("7,266 requests");
  });

  it("resolves native plural keys from a numeric count", () => {
    const { result } = renderHook(() => useTranslations());

    expect(result.current.t("webhooks.selectedCount", { count: 1 })).toBe(
      "1 event selected",
    );
    expect(result.current.t("webhooks.selectedCount", { count: 4 })).toBe(
      "4 events selected",
    );
  });

  it("follows the current i18next language", async () => {
    const { result } = renderHook(() => useTranslations());

    await act(async () => {
      await result.current.i18n.changeLanguage("zh");
    });

    expect(result.current.t("common.appName")).toBe("bex");
    expect(result.current.t("common.navProjects")).toBe("项目");
  });

  describe("dev-only key validation", () => {
    let errorSpy: ReturnType<typeof vi.spyOn>;
    let warnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
      warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    });

    afterEach(() => {
      errorSpy.mockRestore();
      warnSpy.mockRestore();
    });

    it("logs an error for an unprefixed key", () => {
      const { result } = renderHook(() => useTranslations());

      result.current.t("appName");

      expect(errorSpy).toHaveBeenCalledOnce();
      expect(errorSpy.mock.calls[0][0]).toMatch(/missing a namespace prefix/);
    });

    it("logs a warning for a key not present in the en resources", () => {
      const { result } = renderHook(() => useTranslations());

      result.current.t("common.doesNotExist");

      expect(warnSpy).toHaveBeenCalledOnce();
      expect(warnSpy.mock.calls[0][0]).toMatch(
        /was not found in the en resources/,
      );
    });

    it("does not warn for a valid, namespaced key", () => {
      const { result } = renderHook(() => useTranslations());

      result.current.t("common.appName");

      expect(errorSpy).not.toHaveBeenCalled();
      expect(warnSpy).not.toHaveBeenCalled();
    });

    it("does not warn for a plural base key called with a numeric count", () => {
      const { result } = renderHook(() => useTranslations());

      // Only "deploys.listCount_one"/"_other" exist in the catalog; the base
      // key is how native plurals are called (w6/062).
      result.current.t("deploys.listCount", { count: 3 });

      expect(errorSpy).not.toHaveBeenCalled();
      expect(warnSpy).not.toHaveBeenCalled();
    });

    it("warns when a plural key is called without a numeric count", () => {
      const { result } = renderHook(() => useTranslations());

      result.current.t("deploys.listCount");

      expect(warnSpy).toHaveBeenCalledOnce();
      expect(warnSpy.mock.calls[0][0]).toMatch(
        /pluralized \(_one\/_other\) and needs a numeric/,
      );
    });
  });
});
