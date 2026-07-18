import type { TranslationEntry } from "@/i18n";

const enBlueprints: Record<string, TranslationEntry> = {
  "blueprints.resourceType": {
    message: "Blueprint",
    description: "Blueprint resource type used in document titles",
  },
  // --- List page ---
  "blueprints.pageTitle": {
    message: "Blueprints",
    description: "Blueprints list page heading and document title",
  },
  "blueprints.cardTitle": {
    message: "Blueprints",
    description: "Blueprints table card title",
  },
  "blueprints.colName": {
    message: "Name",
    description:
      "Blueprints table column header — blueprint name (derived from repo)",
  },
  "blueprints.colRepo": {
    message: "Repository",
    description: "Blueprints table column header — source repo URL",
  },
  "blueprints.colBranch": {
    message: "Branch",
    description: "Blueprints table column header — tracked branch",
  },
  "blueprints.colStatus": {
    message: "Status",
    description: "Blueprints table column header",
  },
  "blueprints.colUpdated": {
    message: "Last Updated",
    description: "Blueprints table column header — updatedAt relative age",
  },
  // --- Empty state ---
  "blueprints.emptyTitle": {
    message: "No blueprints yet",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "Blueprints auto-register whenever you deploy a repo-backed bex.yml. Once deployed, your stack appears here and you can sync or validate it.",
    description:
      "Blueprints list empty-state body explaining auto-registration",
  },
  // --- Loading / error ---
  "blueprints.loadingBody": {
    message: "Loading blueprints…",
    description: "Blueprints list loading state label",
  },
  "blueprints.errorTitle": {
    message: "Couldn't load blueprints",
    description: "Blueprints list error state heading",
  },
  // --- Status badges ---
  "blueprints.statusActive": {
    message: "Active",
    description: "Blueprint status badge — row is active",
  },
  "blueprints.statusUnknown": {
    message: "{status}",
    description: "Blueprint status badge fallback — raw status value",
  },
  // --- Detail page ---
  "blueprints.detailTitle": {
    message: "{name} · Blueprints",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "Repository",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaBranch": {
    message: "Branch",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaCreated": {
    message: "Created",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Last synced",
    description: "Blueprint detail metadata label",
  },
  "blueprints.manifestTitle": {
    message: "bex.yml manifest",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.notFoundTitle": {
    message: "Blueprint not found",
    description: "Blueprint detail not-found state heading",
  },
  "blueprints.notFoundBody": {
    message: 'No blueprint with id "{id}" exists in this workspace.',
    description: "Blueprint detail not-found state body",
  },
  // --- Sync action ---
  "blueprints.syncButton": {
    message: "Sync",
    description: "Button label that triggers an idempotent blueprint re-apply",
  },
  "blueprints.syncConfirmTitle": {
    message: "Sync blueprint?",
    description: "Sync confirm dialog title",
  },
  "blueprints.syncConfirmBody": {
    message:
      "This re-applies the stored bex.yml to your workspace. The apply is idempotent — resources that already match the manifest are not replaced.",
    description: "Sync confirm dialog description",
  },
  "blueprints.syncConfirmAction": {
    message: "Sync",
    description: "Sync confirm dialog primary action button label",
  },
  "blueprints.syncCancel": {
    message: "Cancel",
    description: "Sync confirm dialog cancel button label",
  },
  "blueprints.syncSuccess": {
    message: "Blueprint synced",
    description: "Toast shown after a successful syncBlueprint call",
  },
  "blueprints.syncError": {
    message: "Sync failed",
    description: "Toast shown when syncBlueprint returns an error",
  },
  // --- Validate action ---
  "blueprints.validateTitle": {
    message: "Validate",
    description: "Validate panel section heading",
  },
  "blueprints.validateRun": {
    message: "Run validate",
    description: "Validate panel button label",
  },
  "blueprints.validateValid": {
    message: "Manifest is valid — no errors found.",
    description: "Validate result: manifest parsed successfully",
  },
  "blueprints.validateInvalid": {
    message: "Manifest has errors:",
    description:
      "Validate result: manifest has parse errors — followed by the error list",
  },
  "blueprints.validateNoResult": {
    message: "No result yet.",
    description: "Validate panel placeholder before first run",
  },
};

export default enBlueprints;
