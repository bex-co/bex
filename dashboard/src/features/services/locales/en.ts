import type { TranslationEntry } from "@/i18n";

const enServices: Record<string, TranslationEntry> = {
  "services.statTotal": {
    message: "Total services",
    description: "Services page stat card label",
  },
  "services.statRunning": {
    message: "Running",
    description: "Services page stat card label",
  },
  "services.statSuspended": {
    message: "Suspended",
    description: "Services page stat card label",
  },
  "services.cardTitle": {
    message: "Services",
    description: "Services table card title, also used as the metrics page back-link",
  },
  "services.sampleDataNotice": {
    message: "Sample data — this scaffold isn't wired to bex-api yet.",
    description: "Services table card description noting the data is a placeholder",
  },
  "services.colName": {
    message: "Name",
    description: "Services table column header",
  },
  "services.colStatus": {
    message: "Status",
    description: "Services table column header",
  },
  "services.colUrl": {
    message: "URL",
    description: "Services table column header",
  },
  "services.statusRunning": {
    message: "running",
    description: "Services table status badge",
  },
  "services.statusSuspended": {
    message: "suspended",
    description: "Services table status badge",
  },
};

export default enServices;
