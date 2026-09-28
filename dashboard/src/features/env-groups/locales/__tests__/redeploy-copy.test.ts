import { describe, expect, it } from "vitest";

import en from "../en";
import zh from "../zh";

/**
 * The three env-group warnings that describe a redeploy must describe the
 * auto-deploy gate, because the gate is real and narrower than "every service".
 *
 * `autoDeployGated` in lego/backend/internal/envgroups/service.go:1163 is
 * `Repo != "" && !AutoDeploy`. So a group write redeploys linked services
 * EXCEPT a repo-backed one whose owner turned Auto-Deploy off: for that service
 * the group's Secret refs still land, but `spec.restartedAt` is untouched and no
 * deploy row opens, so it keeps serving its current release. Image-backed
 * services are deliberately NOT gated — their `autoDeploy: false` is a default
 * (no branch to watch), not an opt-out.
 *
 * w1/113: all three strings promised an unconditional redeploy of "every"
 * linked service. `w2/m94`'s live re-probe (pass 225) linked a group to a
 * service with auto-deploy off, no deploy row opened, and the copy was wrong in
 * exactly the case the gate exists for. Nothing pinned the copy, so it drifted
 * from the behavior for as long as the gate had existed.
 */
const REDEPLOY_KEYS = [
  "envGroups.varDeleteConfirmBody",
  "envGroups.fileDeleteConfirmBody",
  "envGroups.servicesDescription",
  // Not named by w1/113, but the census below caught it and it is the same
  // overpromise in the present tense: only non-gated services are redeploying.
  "envGroups.rolloutNote",
] as const;

const CATALOGS = [
  { name: "en", catalog: en as Record<string, { message: string }> },
  { name: "zh", catalog: zh as Record<string, { message: string }> },
] as const;

describe("env-group redeploy copy names the auto-deploy gate", () => {
  it("covers exactly the keys that promise a redeploy", () => {
    // Guards the list itself: a new redeploy warning must be added here rather
    // than silently escaping the assertions below.
    const promising = Object.entries(
      en as Record<string, { message: string }>,
    ).filter(([, v]) => /redeploy/i.test(v.message));
    expect(promising.map(([k]) => k).sort()).toEqual([...REDEPLOY_KEYS].sort());
  });

  describe.each(CATALOGS)("$name", ({ name, catalog }) => {
    it.each(REDEPLOY_KEYS)(
      "%s conditions the redeploy on auto-deploy",
      (key) => {
        const message = catalog[key]?.message;
        expect(message, `${name} is missing ${key}`).toBeTruthy();

        // The gate must be named, in whichever language.
        const namesGate =
          name === "en"
            ? /auto-deploy/i.test(message)
            : /自动部署/.test(message);
        expect(namesGate, `${name} ${key} does not name auto-deploy`).toBe(
          true,
        );

        // And the unconditional promise must be gone.
        const absolute =
          name === "en"
            ? /every linked service will redeploy|redeploys every affected service/i.test(
                message,
              )
            : /所有关联服务都将.*重新部署|重新部署每个受影响的服务/.test(
                message,
              );
        expect(
          absolute,
          `${name} ${key} still promises an unconditional redeploy`,
        ).toBe(false);
      },
    );

    it.each(REDEPLOY_KEYS)(
      "%s does not claim auto-deploy off blocks the value change",
      (key) => {
        // The Secret refs still land for a gated service, so the change is not
        // prevented — only the rollout is deferred. Copy must not overpromise
        // in the other direction while fixing the first overpromise.
        const message = catalog[key]!.message;
        const overpromises =
          name === "en"
            ? /(will not|won't|never) (be )?(change|appl|receiv|updat)/i.test(
                message,
              )
            : /(不会|永不).*(更改|应用|更新)/.test(message);
        expect(
          overpromises,
          `${name} ${key} claims auto-deploy off prevents the change`,
        ).toBe(false);
      },
    );
  });
});
