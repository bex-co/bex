import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  AUTO_DEPLOY_DISABLED_REASONS,
  autoDeployDisabledReasonKey,
  eventReasonKey,
  memoryLimitLabel,
} from "@/features/services/lib/event-reason";
import en from "@/features/services/locales/en";
import zh from "@/features/services/locales/zh";

// w8/m158: a running service's process that crashes, is killed for memory or
// ends on its own is an Instance Failed event with that cause. The existing
// strings for those codes say "The run …" and belong to a cron run.
describe("eventReasonKey", () => {
  it("words a service's exits as the instance's, and a cron run's as the run's", () => {
    expect(eventReasonKey("server_failed", "non_zero_exit")).toBe(
      "services.eventsReason.instance.non_zero_exit",
    );
    // Only a running service ends early, so that code has one sentence.
    expect(eventReasonKey("server_failed", "early_exit")).toBe(
      "services.eventsReason.early_exit",
    );
    expect(eventReasonKey("cron_job_run_ended", "non_zero_exit")).toBe(
      "services.eventsReason.non_zero_exit",
    );
    // A failed health check is not an exit.
    expect(eventReasonKey("server_failed", "readiness_failed")).toBe(
      "services.eventsReason.readiness_failed",
    );
  });

  it("names the memory limit only when it is known", () => {
    expect(eventReasonKey("server_failed", "oom_killed", "512Mi")).toBe(
      "services.eventsReason.instance.oom_killed",
    );
    expect(eventReasonKey("server_failed", "oom_killed", null)).toBe(
      "services.eventsReason.instance.oom_killed_no_limit",
    );
  });

  it("has every key it can return in both locales", () => {
    const keys = [
      eventReasonKey("server_failed", "non_zero_exit"),
      eventReasonKey("server_failed", "oom_killed", "512Mi"),
      eventReasonKey("server_failed", "oom_killed"),
      eventReasonKey("server_failed", "early_exit"),
    ];
    for (const key of keys) {
      expect(en).toHaveProperty([key]);
      expect(zh).toHaveProperty([key]);
    }
  });
});

describe("memoryLimitLabel", () => {
  it("reads a Kubernetes quantity as megabytes or gigabytes", () => {
    expect(memoryLimitLabel("512Mi")).toBe("512 MB");
    expect(memoryLimitLabel("2Gi")).toBe("2 GB");
    expect(memoryLimitLabel("1.5Gi")).toBe("1.5 GB");
    expect(memoryLimitLabel("weird")).toBe("weird");
  });
});

// w4/m431: every reason the API can send has a sentence in every shipped
// language. Read from the Go constants, so a new backend reason without
// dashboard wording fails here rather than rendering a bare title.
describe("autoDeployDisabledReasonKey", () => {
  const audit = readFileSync(
    `${process.cwd()}/../lego/backend/internal/core/audit.go`,
    "utf8",
  );
  const backendReasons = [
    ...audit.matchAll(/^\s*AutoDeployReason[A-Za-z]+\s*=\s*"([^"]+)"/gm),
  ].map(([, value]) => value);

  it("covers exactly the reasons the backend writes", () => {
    expect(backendReasons).toContain("manual_deploy");
    expect([...AUTO_DEPLOY_DISABLED_REASONS].sort()).toEqual(
      [...backendReasons].sort(),
    );
  });

  it("has a sentence for each reason in en and zh, and none for an unknown one", () => {
    for (const reason of AUTO_DEPLOY_DISABLED_REASONS) {
      const key = autoDeployDisabledReasonKey(reason);
      expect(key, reason).not.toBeNull();
      expect(en[key as keyof typeof en], reason).toBeDefined();
      expect(zh[key as keyof typeof zh], reason).toBeDefined();
    }
    expect(autoDeployDisabledReasonKey("some_new_reason")).toBeNull();
    expect(autoDeployDisabledReasonKey(null)).toBeNull();
  });
});
