import { describe, expect, it } from "vitest";

import {
  deployStatusKey,
  deployStatusVariant,
  isTerminalDeployStatus,
} from "./deploy-status";

describe("deployStatusKey", () => {
  it("maps the supported deploy statuses to namespaced translation keys", () => {
    expect(deployStatusKey("live")).toBe("deploys.statusLive");
    expect(deployStatusKey("update_in_progress")).toBe(
      "deploys.statusUpdateInProgress",
    );
    expect(deployStatusKey("update_failed")).toBe("deploys.statusUpdateFailed");
    expect(deployStatusKey("canceled")).toBe("deploys.statusCanceled");
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
