import { afterEach, describe, expect, it } from "vitest";
import { en, loadLanguageResources } from "@/i18n";
import i18n from "@/i18n/init";
import { translator } from "@/test/translator";
import {
  deployReasonText,
  toDeployReasonTemplate,
} from "@/features/deploys/lib/deploy-reason";

type Param = { name: string; value: string };
const wire = (
  code: string,
  params: Param[],
  parts: { code: string; params: Param[]; text: string }[] = [],
  remainder = "",
) => toDeployReasonTemplate({ code, params, parts, remainder });

// w4/566: a reason bex-api names by code reads in the dashboard's language;
// in en exactly as bex-api words it, and an unknown code keeps its sentence.
describe("deployReasonText", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });
  const detected = wire("http_port_detected", [{ name: "port", value: "80" }]);
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
    const superseded = wire("superseded_by", [
      { name: "deploy", value: "dep-d4newer" },
    ]);
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
    const stallParams = [
      { name: "kind", value: "startup" },
      { name: "path", value: "/healthz" },
      { name: "port", value: "3000" },
      { name: "restarts", value: "3" },
    ];
    const stall = wire("probe_http_failed_restarted", [
      ...stallParams,
      { name: "hint", value: "" },
    ]);
    const sentence =
      "the container is running but its startup health check never succeeded within the rollout window, so the deploy failed: GET /healthz on port 3000." +
      " The liveness check has already restarted the container 3 time(s), so the check is failing rather than merely slow." +
      " Check that the path returns a 2xx or 3xx status on the service's own port.";
    const zhSentence =
      "容器正在运行，但其启动健康检查在发布时限内始终未成功，部署因此失败：端口 3000 上的 GET /healthz。" +
      "存活检查已重启容器 3 次，说明检查确实在失败，而不只是响应慢。" +
      "请确认该路径在服务自身端口上返回 2xx 或 3xx 状态码。";

    it("reads in en exactly as bex-api's sentence", () => {
      expect(deployReasonText(translator("en"), sentence, stall)).toBe(
        sentence,
      );
    });

    it("words the check's kind in zh", () => {
      expect(deployReasonText(translator("zh"), sentence, stall)).toBe(
        zhSentence,
      );
    });

    // w5/m356: the port hints bex-api read as parts are worded one by one,
    // so the whole stall reads in Chinese; en is still the API's sentence.
    describe("with port hints", () => {
      const noOpen =
        " No open port was detected on the instance: the server must bind host 0.0.0.0 on the port in PORT (10000); it listened on port 3000 before.";
      const worker =
        " If the service does not need to receive traffic on any port, create a background worker instead.";
      const hinted = wire(
        "probe_http_failed_restarted",
        [...stallParams, { name: "hint", value: noOpen + worker }],
        [
          {
            code: "hint_no_open_port_before",
            params: [
              { name: "port", value: "10000" },
              { name: "previous", value: "3000" },
            ],
            text: noOpen,
          },
          { code: "hint_background_worker", params: [], text: worker },
        ],
      );

      it("reads in en exactly as bex-api's sentence", () => {
        expect(
          deployReasonText(
            translator("en"),
            sentence + noOpen + worker,
            hinted,
          ),
        ).toBe(sentence + noOpen + worker);
      });

      it("words every hint in zh", () => {
        expect(
          deployReasonText(
            translator("zh"),
            sentence + noOpen + worker,
            hinted,
          ),
        ).toBe(
          zhSentence +
            "实例上未检测到开放端口：服务器必须绑定到 0.0.0.0，并监听 PORT 环境变量指定的端口（10000）；它之前监听的是端口 3000。" +
            "如果服务不需要在任何端口上接收流量，请改为创建后台工作进程。",
        );
      });

      // A hint the dashboard does not word yet reads in bex-api's English,
      // and the tail no template words stays English after the worded ones.
      it("keeps an unworded hint and the remainder in English", () => {
        const unknown = " Something bex words in a newer release.";
        const remainder = " A sentence no template words.";
        const mixed = wire(
          "probe_http_failed_restarted",
          [
            ...stallParams,
            { name: "hint", value: worker + unknown + remainder },
          ],
          [
            { code: "hint_background_worker", params: [], text: worker },
            { code: "hint_from_the_future", params: [], text: unknown },
          ],
          remainder,
        );
        const full = sentence + worker + unknown + remainder;
        expect(deployReasonText(translator("en"), full, mixed)).toBe(full);
        expect(deployReasonText(translator("zh"), full, mixed)).toBe(
          zhSentence +
            "如果服务不需要在任何端口上接收流量，请改为创建后台工作进程。" +
            unknown +
            remainder,
        );
      });
    });

    // A hint no template words at all is the remainder: it stays English
    // after the worded sentence, so the sentence's language never flips.
    it("keeps an unworded hint after the zh sentence", () => {
      const hint = " No open port was detected on the instance.";
      const hinted = wire(
        "probe_http_failed_restarted",
        [...stallParams, { name: "hint", value: hint }],
        [],
        hint,
      );
      expect(deployReasonText(translator("en"), sentence + hint, hinted)).toBe(
        sentence + hint,
      );
      expect(deployReasonText(translator("zh"), sentence + hint, hinted)).toBe(
        zhSentence + hint,
      );
    });
  });

  // w5/m356: a never-started build's cause is worded too.
  describe("a build that never started", () => {
    const cause =
      "workspace has 2/2 concurrent builds active; waiting for a slot";
    const sentence = "the build never started: " + cause;
    const queued = wire(
      "build_never_started",
      [{ name: "cause", value: cause }],
      [
        {
          code: "queued_build_limit",
          params: [
            { name: "noun", value: "workspace" },
            { name: "active", value: "2" },
            { name: "limit", value: "2" },
          ],
          text: cause,
        },
      ],
    );

    it("reads in en exactly as bex-api's sentence", () => {
      expect(deployReasonText(translator("en"), sentence, queued)).toBe(
        sentence,
      );
    });

    it("words its cause in zh", () => {
      expect(deployReasonText(translator("zh"), sentence, queued)).toBe(
        "构建一直未开始：工作区已有 2/2 个并发构建在运行；正在等待空位",
      );
    });

    // Kubernetes' own words in a capacity wait stay English.
    it("keeps the scheduler's verdict in English", () => {
      const detail = "0/7 nodes are available: 7 Insufficient cpu";
      const capacity = "waiting for build capacity: " + detail;
      const reason = wire(
        "build_never_started",
        [{ name: "cause", value: capacity }],
        [
          {
            code: "queued_capacity",
            params: [{ name: "detail", value: detail }],
            text: capacity,
          },
        ],
      );
      expect(
        deployReasonText(
          translator("en"),
          "the build never started: " + capacity,
          reason,
        ),
      ).toBe("the build never started: " + capacity);
      expect(
        deployReasonText(
          translator("zh"),
          "the build never started: " + capacity,
          reason,
        ),
      ).toBe("构建一直未开始：正在等待构建资源：" + detail);
    });
  });

  // w5/m356: a deferred restart's wait reads as a duration in zh, and as
  // bex-api's Go duration in en.
  describe("a restart that waited too long", () => {
    it.each([
      ["3h0m0s", "3 小时"],
      ["45m0s", "45 分钟"],
      ["1h30m0s", "1 小时 30 分钟"],
      ["1m30s", "1 分钟 30 秒"],
    ])("words %s", (wait, words) => {
      const sentence = `the deploy this restart waited for did not finish within ${wait}`;
      const reason = wire("restart_wait_timed_out", [
        { name: "wait", value: wait },
      ]);
      expect(deployReasonText(translator("en"), sentence, reason)).toBe(
        sentence,
      );
      expect(deployReasonText(translator("zh"), sentence, reason)).toBe(
        `这次重启等待的部署未在 ${words}内完成`,
      );
    });

    it("keeps a wait in any other shape as-is", () => {
      const reason = wire("restart_wait_timed_out", [
        { name: "wait", value: "500ms" },
      ]);
      expect(deployReasonText(translator("zh"), "", reason)).toBe(
        "这次重启等待的部署未在 500ms内完成",
      );
    });
  });

  it("keeps bex-api's sentence for an unknown code or none", () => {
    const unknown = wire("something_new", []);
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
