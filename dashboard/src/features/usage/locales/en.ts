import type { TranslationEntry } from "@/i18n";

const enUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "Usage",
    description: "Usage page heading and browser title",
  },
  "usage.pageSubtitle": {
    message: "Month-to-date workspace consumption",
    description: "Usage page subtitle beneath the heading",
  },
  "usage.computeTitle": {
    message: "Compute",
    description: "Compute section heading on the Usage page",
  },
  "usage.computeDescription": {
    message: "Instance-hours by service and plan",
    description: "Compute section description",
  },
  "usage.bandwidthTitle": {
    message: "Bandwidth",
    description: "Bandwidth section heading on the Usage page",
  },
  "usage.bandwidthDescription": {
    message: "Outbound egress metered by Traefik",
    description: "Bandwidth section description",
  },
  "usage.buildTitle": {
    message: "Build Minutes",
    description: "Build minutes section heading on the Usage page",
  },
  "usage.buildDescription": {
    message: "Pipeline minutes consumed by builds",
    description: "Build section description",
  },
  "usage.colService": {
    message: "Service",
    description: "Table column header: service name",
  },
  "usage.colPlan": {
    message: "Plan",
    description: "Table column header: service plan/tier",
  },
  "usage.colHours": {
    message: "Hours",
    description: "Table column header: compute hours",
  },
  "usage.colBandwidth": {
    message: "Bandwidth",
    description: "Table column header: egress bandwidth",
  },
  "usage.colMinutes": {
    message: "Minutes",
    description: "Table column header: build minutes",
  },
  "usage.totalRow": {
    message: "Total",
    description: "Summary row label in usage tables",
  },
  "usage.empty": {
    message: "No usage recorded this month.",
    description: "Empty-state message when a section has no data",
  },
  "usage.errorTitle": {
    message: "Could not load usage",
    description: "Error state heading on the Usage page",
  },
};

export default enUsage;
