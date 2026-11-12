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

  // w4/591: a cancel reason's deploy id reads as-is inside the zh sentence.
  it("words a cancel reason in zh, keeping the deploy id", () => {
    const superseded = toDeployReasonTemplate({
      code: "superseded_by",
      params: [{ name: "deploy", value: "dep-d4newer" }],
    });
    expect(
      deployReasonText(
        translator("zh"),
        "Superseded by dep-d4newer",
        superseded,
      ),
    ).toBe("已被 dep-d4newer 取代");
  });

  // A probe stall's {kind} is an English word inside the sentence; the
  // catalog words it too, and en still reads exactly as bex-api's sentence.
  describe("a health-check stall", () => {
    const stall = toDeployReasonTemplate({
      code: "probe_http_failed_restarted",
      params: [
        { name: "kind", value: "startup" },
        { name: "path", value: "/healthz" },
        { name: "port", value: "3000" },
        { name: "restarts", value: "3" },
        { name: "hint", value: "" },
      ],
    });
    const sentence =
      "the container is running but its startup health check never succeeded within the rollout window, so the deploy failed: GET /healthz on port 3000." +
      " The liveness check has already restarted the container 3 time(s), so the check is failing rather than merely slow." +
      " Check that the path returns a 2xx or 3xx status on the service's own port.";

    it("reads in en exactly as bex-api's sentence", () => {
      expect(deployReasonText(translator("en"), sentence, stall)).toBe(
        sentence,
      );
    });

    it("words the check's kind in zh", () => {
      expect(deployReasonText(translator("zh"), sentence, stall)).toBe(
        "容器正在运行，但其启动健康检查在发布时限内始终未成功，部署因此失败：端口 3000 上的 GET /healthz。" +
          "存活检查已重启容器 3 次，说明检查确实在失败，而不只是响应慢。" +
          "请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。",
      );
    });

    // A port hint the operator appends on some passes stays English after
    // the worded sentence, so the sentence's language never flips.
    it("keeps an appended port hint after the zh sentence", () => {
      const hint = " No open port was detected on the instance.";
      const hinted = toDeployReasonTemplate({
        code: "probe_http_failed_restarted",
        params: [
          { name: "kind", value: "startup" },
          { name: "path", value: "/healthz" },
          { name: "port", value: "3000" },
          { name: "restarts", value: "3" },
          { name: "hint", value: hint },
        ],
      });
      expect(deployReasonText(translator("en"), sentence + hint, hinted)).toBe(
        sentence + hint,
      );
      expect(
        deployReasonText(translator("zh"), sentence + hint, hinted),
      ).toMatch(
        /^容器正在运行.*状态码。 No open port was detected on the instance\.$/,
      );
    });
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
