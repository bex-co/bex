import { useMemo, useState } from "react";
import { useRouter } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { useRepoBranches } from "@/features/services/hooks/use-repo-branches";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import {
  useUpdateBlueprint,
  type BlueprintUpdate,
} from "@/features/blueprints/hooks/use-update-blueprint";
import { blueprintSettingsRefusal } from "@/features/blueprints/lib/settings-refusal";
import type { BlueprintView } from "@/features/blueprints/types";

/** Client-side mirror of the backend's approvedBlueprintPath rule (m19):
 * clean repo-relative .yaml/.yml, no escapes. The backend re-validates. */
function validBlueprintPath(path: string): boolean {
  const p = path.trim();
  if (!p || p.startsWith("/") || p.includes("\\")) return false;
  if (p.split("/").some((seg) => seg === "" || seg === "." || seg === "..")) {
    return false;
  }
  return /\.(yaml|yml)$/.test(p);
}

type SettingsField = keyof BlueprintUpdate;

/** A row's refused value and what it said, until the value changes. */
interface Refusal {
  field: SettingsField;
  value: string;
  message: string;
}

/**
 * Render's Settings card for a Blueprint (w3/m679): Blueprint Name, Auto
 * Sync, Branch and Blueprint Path, in Render's order and words, each an
 * edit-in-place row that saves only its own field. A refused save keeps the
 * row open with Render's sentence under it. A role that cannot change the
 * Blueprint (bex-api checks can_create, as Render checks updateBlueprint)
 * sees the rows read-only with the reason. Render's Sync Hook (shown only
 * with a sync key) and Git Credentials (#GIT-WORKSPACE-OWNED) are not drawn.
 */
export function BlueprintSettingsCard({
  blueprint,
}: {
  blueprint: BlueprintView;
}) {
  const { t } = useTranslations();
  const router = useRouter();
  const { update, busy } = useUpdateBlueprint();
  const gate = useCapabilityGate("canCreate");
  // Read the repository's branches once Branch is edited, as Render's
  // picker does, not on every visit.
  const [branchPicking, setBranchPicking] = useState(false);
  const { branches } = useRepoBranches(branchPicking ? blueprint.repo : null);
  const branchOptions = useMemo(
    () => branches.map((value) => ({ value, label: value })),
    [branches],
  );
  const [refusal, setRefusal] = useState<Refusal | null>(null);

  // Render reads Auto Sync as "not paused"; a paused Blueprint reads No.
  const autoSync = blueprint.autoSync && blueprint.status !== "paused";

  /** Saves one row's field; a refusal is worded for the file the save
   *  would read: its new path or branch, else the Blueprint's own. */
  async function save(
    field: SettingsField,
    patch: BlueprintUpdate,
  ): Promise<boolean> {
    setRefusal(null);
    const result = await update(blueprint.id, patch);
    if (!result.ok) {
      const source = {
        path: patch.path ?? blueprint.path,
        branch: patch.branch ?? blueprint.branch,
      };
      setRefusal({
        field,
        value: String(patch[field]),
        message: blueprintSettingsRefusal(result.error, source, t),
      });
      return false;
    }
    // The page title and the layout's loader read the name too.
    void router.invalidate();
    return true;
  }

  /** The refusal for `field` while its draft is still the refused value. */
  function refusedAt(field: SettingsField, draft: string): string | null {
    return refusal?.field === field && refusal.value === draft.trim()
      ? refusal.message
      : null;
  }

  const rowProps = {
    busy,
    disabled: gate.denied,
    disabledReason: gate.reason,
  };
  const required = (draft: string) =>
    draft.trim() === "" ? t("blueprints.settingsRequired") : null;
  const nameLabel = t("blueprints.createNameLabel");
  const autoSyncLabel = t("blueprints.settingsAutoSync");
  const branchLabel = t("blueprints.settingsBranch");
  const pathLabel = t("blueprints.settingsPath");

  return (
    <Card>
      <CardHeader>
        {/* Render's card title, which heads the page (the layout draws no
            heading of its own for Settings). */}
        <CardTitle className="text-base">
          {t("blueprints.navSettings")}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <EditableFieldRow
          {...rowProps}
          label={nameLabel}
          hint={t("blueprints.settingsNameHint")}
          value={blueprint.name}
          editLabel={t("blueprints.editField", { field: nameLabel })}
          validate={(draft) => required(draft) ?? refusedAt("name", draft)}
          onSave={(name) => save("name", { name })}
        />
        <EditableFieldRow
          {...rowProps}
          label={autoSyncLabel}
          hint={t("blueprints.settingsAutoSyncHint")}
          value={String(autoSync)}
          editLabel={t("blueprints.editField", { field: autoSyncLabel })}
          options={[
            { value: "true", label: t("blueprints.settingsYes") },
            { value: "false", label: t("blueprints.settingsNo") },
          ]}
          validate={(draft) => refusedAt("autoSync", draft)}
          onSave={(value) => save("autoSync", { autoSync: value === "true" })}
        />
        <EditableFieldRow
          {...rowProps}
          label={branchLabel}
          hint={t("blueprints.settingsBranchHint")}
          value={blueprint.branch}
          editLabel={t("blueprints.editField", { field: branchLabel })}
          comboboxOptions={branchOptions}
          onEdit={() => setBranchPicking(true)}
          validate={(draft) => required(draft) ?? refusedAt("branch", draft)}
          onSave={(branch) => save("branch", { branch })}
        />
        <EditableFieldRow
          {...rowProps}
          label={pathLabel}
          hint={t("blueprints.settingsPathHint")}
          value={blueprint.path}
          placeholder="render.yaml"
          mono
          editLabel={t("blueprints.editField", { field: pathLabel })}
          validate={(draft) =>
            validBlueprintPath(draft)
              ? refusedAt("path", draft)
              : t("blueprints.pathInvalid")
          }
          onSave={(path) => save("path", { path })}
        />
      </CardContent>
    </Card>
  );
}
