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

    expect(result.current.t("metrics.responseTimes", { quantile: 0.95 })).toBe(
      "Response Times (0.95)",
    );
  });

  it("follows the current i18next language", async () => {
    const { result } = renderHook(() => useTranslations());

    await act(async () => {
      await result.current.i18n.changeLanguage("zh");
    });

    expect(result.current.t("common.appName")).toBe("bex");
    expect(result.current.t("common.navServices")).toBe("服务");
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

      // @ts-expect-error -- intentionally passing a key missing its namespace prefix
      result.current.t("appName");

      expect(errorSpy).toHaveBeenCalledOnce();
      expect(errorSpy.mock.calls[0][0]).toMatch(/missing a namespace prefix/);
    });

    it("logs a warning for a key not present in the en resources", () => {
      const { result } = renderHook(() => useTranslations());

      // @ts-expect-error -- intentionally passing a key that doesn't exist
      result.current.t("common.doesNotExist");

      expect(warnSpy).toHaveBeenCalledOnce();
      expect(warnSpy.mock.calls[0][0]).toMatch(/was not found in the en resources/);
    });

    it("does not warn for a valid, namespaced key", () => {
      const { result } = renderHook(() => useTranslations());

      result.current.t("common.appName");

      expect(errorSpy).not.toHaveBeenCalled();
      expect(warnSpy).not.toHaveBeenCalled();
    });
  });
});
