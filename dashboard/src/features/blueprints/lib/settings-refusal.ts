import type { Translate } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  relayableReason,
} from "@/common/lib/graphql-error";

/** The Blueprint file a refused Settings save would have read. */
export interface BlueprintFileSource {
  path: string;
  branch: string;
}

/**
 * What a Settings row says when bex-api refuses its save, in Render's words
 * (w3/m679, docs/render-artifacts/blueprint-pages.md § Settings): a name
 * another Blueprint has, a branch the repository lacks, the Blueprint file
 * missing from `source` (the path the save would read, on its branch), or
 * else the server's own reason, then Render's "Failed to update Blueprint.
 * Please try again." when the server gave none.
 */
export function blueprintSettingsRefusal(
  error: unknown,
  source: BlueprintFileSource,
  t: Translate,
): string {
  if (hasGraphQLErrorCode(error, "BLUEPRINT_NAME_TAKEN")) {
    return t("blueprints.settingsNameTaken");
  }
  if (hasGraphQLErrorCode(error, "branch_not_found")) {
    return t("blueprints.settingsBranchNotFound");
  }
  if (hasGraphQLErrorCode(error, "file_not_found")) {
    return t("blueprints.settingsFileNotFound", {
      path: source.path || "render.yaml",
      branch: source.branch,
    });
  }
  return relayableReason(error) || t("blueprints.settingsUpdateFailed");
}
