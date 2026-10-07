import type { en } from "@/i18n";

// Manifest kinds, as plan actions and bex-api's ownership conflicts name them
// (w5/098).
const KIND_LABEL: Record<string, keyof typeof en> = {
  service: "blueprints.previewKindService",
  postgres: "blueprints.previewKindPostgres",
  key_value: "blueprints.previewKindKeyValue",
  env_var_group: "blueprints.previewKindEnvGroup",
};

/** The label key of a Blueprint resource kind, undefined for an unknown one. */
export function blueprintKindLabel(kind: string): keyof typeof en | undefined {
  return KIND_LABEL[kind];
}
