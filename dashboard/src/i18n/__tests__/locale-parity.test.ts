import { describe, it, expect } from "vitest";
import enCommon from "@/common/locales/en";
import zhCommon from "@/common/locales/zh";
import enAuth from "@/features/auth/locales/en";
import zhAuth from "@/features/auth/locales/zh";
import enMetrics from "@/features/metrics/locales/en";
import zhMetrics from "@/features/metrics/locales/zh";
import enServices from "@/features/services/locales/en";
import zhServices from "@/features/services/locales/zh";

const NAMESPACES = [
  { name: "common", en: enCommon, zh: zhCommon },
  { name: "auth", en: enAuth, zh: zhAuth },
  { name: "metrics", en: enMetrics, zh: zhMetrics },
  { name: "services", en: enServices, zh: zhServices },
];

describe("locale key parity", () => {
  it.each(NAMESPACES)("$name: en and zh have the same keys", ({ en, zh }) => {
    const enKeys = Object.keys(en).sort();
    const zhKeys = Object.keys(zh).sort();

    expect(zhKeys).toEqual(enKeys);
  });

  it.each(NAMESPACES)(
    "$name: every key is namespace-prefixed",
    ({ name, en }) => {
      for (const key of Object.keys(en)) {
        expect(key.startsWith(`${name}.`)).toBe(true);
      }
    },
  );

  it.each(NAMESPACES)(
    "$name: every entry has a non-empty message",
    ({ en, zh }) => {
      for (const entry of [...Object.values(en), ...Object.values(zh)]) {
        expect(entry.message.length).toBeGreaterThan(0);
      }
    },
  );
});
