import { describe, expect, it } from "vitest";
import { en } from "@/i18n";
import { sshRemedy } from "@/features/services/lib/ssh-remedy";

// A suspended service's header has no Connect since w3/107, so its Shell
// page is where these remedies show; the copy is what they say there.
describe("sshRemedy", () => {
  const copy = (key: keyof typeof en) => en[key];

  it("points a suspended service at Resume", () => {
    const remedy = sshRemedy({
      type: "web_service",
      suspended: true,
      plan: "starter",
    });
    expect(copy(remedy.reason)).toBe(
      "This service is suspended. Resume it to connect over SSH.",
    );
    expect(remedy.action).toEqual({
      label: "services.sshRemedyResume",
      path: "settings",
    });
    expect(copy(remedy.action!.label)).toBe("Resume service");
  });

  // w4/m183: a suspended Free service has both blockers, and resuming one
  // does not grant SSH, so the copy must not say it does.
  it("names both blockers on a suspended Free service without promising SSH after Resume", () => {
    const remedy = sshRemedy({
      type: "web_service",
      suspended: true,
      plan: "free",
    });
    expect(copy(remedy.reason)).toBe(
      "This service is suspended, and SSH needs a paid compute plan. Resuming restores traffic; Free services still don't include shell access.",
    );
    expect(remedy.action?.path).toBe("settings");
  });

  it("keeps the general explanation for a type no resume gives SSH", () => {
    for (const type of ["static_site", "cron_job"]) {
      expect(sshRemedy({ type, suspended: true, plan: "starter" })).toEqual({
        reason: "services.sshUnavailableHint",
        action: null,
      });
    }
  });
});
