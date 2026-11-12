import { afterEach, describe, expect, it } from "vitest";
import { en, loadLanguageResources } from "@/i18n";
import i18n from "@/i18n/init";
import { translator } from "@/test/translator";
import {
  deployReasonText,
  toDeployReasonTemplate,
} from "@/features/deploys/lib/deploy-reason";

// w4/566: a reason bex-api names by code reads in the dashboard's language;
// in en exactly as bex-api words it, and an unknown code keeps its sentence.
describe("deployReasonText", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });
  const detected = toDeployReasonTemplate({
    code: "http_port_detected",
    params: [{ name: "port", value: "80" }],
  });
  const sentence = "Detected HTTP on port 80; serving it";

  it("reads a known code in en exactly as bex-api's sentence", () => {
    expect(deployReasonText(translator("en"), sentence, detected)).toBe(
      sentence,
    );
  });

  it("words a known code in zh", () => {
    expect(deployReasonText(translator("zh"), sentence, detected)).toBe(
      "已在端口 80 上检测到 HTTP，将通过该端口提供服务",
    );
  });

  it("keeps bex-api's sentence for an unknown code or none", () => {
    const unknown = toDeployReasonTemplate({
      code: "something_new",
      params: [],
    });
    expect(deployReasonText(translator("zh"), "Something new", unknown)).toBe(
      "Something new",
    );
    expect(deployReasonText(translator("zh"), "Plain text", null)).toBe(
      "Plain text",
    );
  });
});

// zh fills the same parameters as en for every template, so no value drops.
it("words every deploy reason in zh with en's parameters", async () => {
  const zh = await loadLanguageResources("zh");
  const params = (message: string) =>
    [...message.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
  const keys = Object.keys(en).filter((k) => k.startsWith("deploys.reason."));
  expect(keys.length).toBeGreaterThan(0);
  for (const key of keys) {
    expect(zh[key], key).toBeDefined();
    expect(params(zh[key]), key).toEqual(params(en[key]));
  }
});
