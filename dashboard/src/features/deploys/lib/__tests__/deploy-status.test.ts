import { describe, expect, it } from "vitest";

import {
  deployStatusKey,
  deployStatusVariant,
  deployTriggerKey,
  isCancelableDeployStatus,
  isRollbackableDeployStatus,
  isTerminalDeployStatus,
} from "../deploy-status";

describe("deployStatusKey", () => {
  it.each([
    ["created", "deploys.statusCreated", false],
    ["queued", "deploys.statusQueued", false],
    ["build_in_progress", "deploys.statusBuildInProgress", false],
    ["build_failed", "deploys.statusBuildFailed", true],
    ["pre_deploy_in_progress", "deploys.statusPreDeployInProgress", false],
    ["pre_deploy_failed", "deploys.statusPreDeployFailed", true],
    ["update_in_progress", "deploys.statusUpdateInProgress", false],
    ["update_failed", "deploys.statusUpdateFailed", true],
    ["live", "deploys.statusLive", true],
    ["deactivated", "deploys.statusDeactivated", true],
    ["canceled", "deploys.statusCanceled", true],
  ] as const)("maps %s to %s and terminal=%s", (status, key, terminal) => {
    expect(deployStatusKey(status)).toBe(key);
    expect(isTerminalDeployStatus(status)).toBe(terminal);
    expect(isCancelableDeployStatus(status)).toBe(!terminal);
  });

  it("classifies terminal failures consistently", () => {
    expect(deployStatusVariant("build_failed")).toBe("destructive");
    expect(deployStatusVariant("pre_deploy_failed")).toBe("destructive");
    expect(isTerminalDeployStatus("build_failed")).toBe(true);
    expect(isTerminalDeployStatus("pre_deploy_failed")).toBe(true);
  });

  it("never returns an empty or unscoped translation key", () => {
    expect(deployStatusKey("")).toBe("deploys.statusUnknown");
    expect(deployStatusKey("surprise")).toBe("deploys.statusUnknown");
  });

  it("offers rollback only to a previous (deactivated) deploy, never the current live one", () => {
    // w4/051: the current live deploy is not a rollback target — rolling back to
    // it is a no-op restart, and the deploys list already excludes it.
    expect(isRollbackableDeployStatus("live")).toBe(false);
    expect(isRollbackableDeployStatus("deactivated")).toBe(true);
    expect(isRollbackableDeployStatus("build_failed")).toBe(false);
    expect(isRollbackableDeployStatus("update_in_progress")).toBe(false);
  });
});

describe("deployTriggerKey", () => {
  it.each([
    ["create", "deploys.triggerCreate"],
    ["api", "deploys.triggerApi"],
    ["deploy_hook", "deploys.triggerDeployHook"],
    ["blueprint", "deploys.triggerBlueprint"],
    ["new_commit", "deploys.triggerNewCommit"],
    ["config_change", "deploys.triggerConfigChange"],
  ] as const)("maps trigger=%s to %s", (trigger, key) => {
    expect(deployTriggerKey(trigger)).toBe(key);
  });

  it("returns null for an unrecognized trigger so the caller can fall back", () => {
    expect(deployTriggerKey("surprise")).toBeNull();
  });
});
