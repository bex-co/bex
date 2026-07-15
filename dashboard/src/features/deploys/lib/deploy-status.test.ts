import { describe, expect, it } from "vitest";

import {
  deployStatusKey,
  deployStatusVariant,
  isCancelableDeployStatus,
  isTerminalDeployStatus,
} from "./deploy-status";

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
});
