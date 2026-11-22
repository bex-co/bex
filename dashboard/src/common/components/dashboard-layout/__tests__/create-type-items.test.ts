import { describe, expect, it } from "vitest";
import {
  SERVICE_TYPES,
  SERVICE_TYPE_CREATE_PATHS,
} from "@/features/services/lib/create-context";
import { CREATE_TYPE_ITEMS, currentCreateType } from "../create-type-items";

describe("CREATE_TYPE_ITEMS", () => {
  // Render's New menu and breadcrumb type switcher (live capture 2026-11-21),
  // less the types bex has no create page for (Workflow, Sandbox Group).
  it("lists every create type once, in Render's order", () => {
    expect(CREATE_TYPE_ITEMS.map((item) => item.id)).toEqual([
      "static_site",
      "web_service",
      "private_service",
      "background_worker",
      "cron_job",
      "postgres",
      "keyvalue",
    ]);
  });

  // Render's own create URLs (live capture 2026-11-21).
  it("links every type to Render's create path", () => {
    const services = CREATE_TYPE_ITEMS.filter((item) => item.service);
    expect(new Set(services.map((item) => item.id))).toEqual(
      new Set(SERVICE_TYPES),
    );
    for (const item of services) {
      expect(item.to).toBe(
        SERVICE_TYPE_CREATE_PATHS[
          item.id as keyof typeof SERVICE_TYPE_CREATE_PATHS
        ],
      );
      expect(item.labelKey).toMatch(/^services\.type/);
    }
    expect(CREATE_TYPE_ITEMS.map((item) => item.to)).toEqual([
      "/static/new",
      "/web/new",
      "/pserv/new",
      "/worker/new",
      "/cron/new",
      "/new/database",
      "/new/redis",
    ]);
  });

  it("names the page each create URL shows", () => {
    expect(currentCreateType("/cron/new")?.id).toBe("cron_job");
    expect(currentCreateType("/pserv/new")?.id).toBe("private_service");
    expect(currentCreateType("/new/database")?.id).toBe("postgres");
    expect(currentCreateType("/new/redis")?.id).toBe("keyvalue");
    expect(currentCreateType("/services/new")).toBeNull();
    expect(currentCreateType("/")).toBeNull();
  });
});
