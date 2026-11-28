import { describe, expect, it } from "vitest";
import { en } from "@/i18n";
import { shellAccess, sshRemedy } from "@/features/services/lib/ssh-remedy";

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

// w3/124: the Shell page's loading frame draws the cards the page will, from
// the service's raw fields.
describe("shellAccess", () => {
  it("follows the service's SSH address when it has been read", () => {
    expect(
      shellAccess({
        type: "web_service",
        plan: "free",
        suspended: "not_suspended",
        sshAddress: "srv-a@ssh.oregon.bex.co",
      }),
    ).toEqual({ eligible: true });
    // No gateway: a paid, running service still has none, so the page says
    // shell access isn't available.
    expect(
      shellAccess({
        type: "web_service",
        plan: "starter",
        suspended: "not_suspended",
        sshAddress: null,
      }),
    ).toEqual({
      eligible: false,
      remedy: { reason: "services.sshUnavailableHint", action: null },
    });
  });

  it.each([
    ["web_service", "starter", "not_suspended", true],
    ["private_service", "pro", "not_suspended", true],
    ["background_worker", "standard", null, true],
    ["web_service", "free", "not_suspended", false],
    ["web_service", "starter", "suspended", false],
    ["web_service", null, "not_suspended", false],
    ["cron_job", "starter", "not_suspended", false],
  ])(
    "predicts %s on %s (%s) as eligible: %s, without a read address",
    (type, plan, suspended, eligible) => {
      expect(shellAccess({ type, plan, suspended })?.eligible).toBe(eligible);
    },
  );

  it("names a suspended service's remedy", () => {
    expect(
      shellAccess({
        type: "background_worker",
        plan: "starter",
        suspended: "suspended",
      }),
    ).toEqual({
      eligible: false,
      remedy: {
        reason: "services.sshUnavailableSuspended",
        action: { label: "services.sshRemedyResume", path: "settings" },
      },
    });
  });

  it("is undecided until the type is known", () => {
    expect(shellAccess(null)).toBeNull();
    expect(shellAccess({ plan: "starter" })).toBeNull();
  });
});
