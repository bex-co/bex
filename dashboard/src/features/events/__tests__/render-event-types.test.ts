import { describe, expect, it } from "vitest";
import {
  eventTypesHiddenBy,
  SERVICE_EVENT_TYPES,
} from "../service-event-catalog";

/** The catalogued types `et` leaves showing. */
function shown(et: string | undefined): string[] {
  const hidden = eventTypesHiddenBy(et);
  return SERVICE_EVENT_TYPES.filter((type) => !hidden.has(type));
}

// w3/m707: Render's Events filter in its URL names types by their GraphQL
// names, which are bex's wire types in PascalCase but for one.
describe("eventTypesHiddenBy (Render's ?et=)", () => {
  it.each([
    ["DeployEnded", "deploy_ended"],
    ["ServerFailed", "server_failed"],
    ["CronJobRunEnded", "cron_job_run_ended"],
    ["MaintenanceModeURIUpdated", "maintenance_mode_uri_updated"],
    ["ExtraInstancesChanged", "instance_count_changed"],
    ["EdgeCachePurged", "edge_cache_purged"],
    ["deploy_started", "deploy_started"],
  ])("shows %s as bex's %s alone", (name, type) => {
    expect(shown(name)).toEqual([type]);
  });

  it("shows each type of a comma-separated list, ignoring spaces and names bex has not", () => {
    expect(
      shown(" DeployStarted , DeployEnded,,MaintenanceStarted"),
    ).toEqual(["deploy_started", "deploy_ended"]);
  });

  it("hides nothing for no value, or one naming no type bex has", () => {
    for (const et of [undefined, "", "MaintenanceStarted", "constructor"]) {
      expect(eventTypesHiddenBy(et).size).toBe(0);
    }
  });
});
