/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useRef, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { setsSensitiveLoggingParameter } from "@/features/databases/lib/sensitive-parameters";
import type { ParameterInput } from "@/graphql/definitions";
import type { SaveParametersResult } from "@/features/databases/hooks/use-database-insights";

interface DraftParameter {
  id: string;
  name: string;
  value: string;
  source: string;
}

/** The subset of the generated `DatabaseParameterOverridesQuery` row this
 * editor reads — structural, so the query result flows in unchanged. */
export interface ParameterOverrideView {
  name: string | null;
  setting: string | null;
  source: string | null;
}

interface ParameterOverridesEditorProps {
  overrides: ParameterOverrideView[];
  saving: boolean;
  onSave: (parameters: ParameterInput[]) => Promise<SaveParametersResult>;
}

function initialDrafts(overrides: ParameterOverrideView[]): DraftParameter[] {
  return overrides
    .filter((override) => Boolean(override.name))
    .map((override, index) => ({
      id: `existing-${index}-${override.name}`,
      name: override.name ?? "",
      value: override.setting ?? "",
      source: override.source ?? "",
    }));
}

function signature(rows: DraftParameter[]): string {
  return JSON.stringify(
    rows
      .map(({ name, value }) => ({ name: name.trim(), value: value.trim() }))
      .sort((a, b) => a.name.localeCompare(b.name)),
  );
}

/** Replace-style editor for Database.spec.parameters, backed by the live view. */
export function ParameterOverridesEditor({
  overrides,
  saving,
  onSave,
}: ParameterOverridesEditorProps) {
  const { t } = useTranslations();
  const { canCreate, canOperate } = useCapabilities();
  const initial = initialDrafts(overrides);
  const [rows, setRows] = useState(initial);
  const [savedRows, setSavedRows] = useState(initial);
  const [saveError, setSaveError] = useState<string | null>(null);
  const nextID = useRef(0);

  const names = rows.map((row) => row.name.trim()).filter(Boolean);
  const duplicate = names.find((name, index) => names.indexOf(name) !== index);
  const hasBlank = rows.some(
    (row) => row.name.trim() === "" || row.value.trim() === "",
  );
  const hasManagedParameter = names.includes("shared_preload_libraries");
  const currentSignature = signature(rows);
  const dirty = currentSignature !== signature(savedRows);

  // The parameter map is a full replacement, so asserting any statement-logging
  // setting makes the update can_create; otherwise it is a can_operate settings
  // change (docs/ADR024, mirrored from the backend). Disable Save with a role
  // reason for a member who lacks the needed relation instead of a 403 on save
  // (w9/m84). Add/remove/edit stay enabled so the member can still see the shape.
  const setsSensitiveLogging = setsSensitiveLoggingParameter(names);
  const permissionBlocked = setsSensitiveLogging ? !canCreate : !canOperate;
  const permissionReason = permissionBlocked
    ? t(
        setsSensitiveLogging
          ? "capabilities.reasonCanCreate"
          : "capabilities.reasonCanOperate",
      )
    : undefined;

  const validationError = hasBlank
    ? t("databases.insightsParamsBlank")
    : duplicate
      ? t("databases.insightsParamsDuplicate", { name: duplicate })
      : hasManagedParameter
        ? t("databases.insightsParamsManaged")
        : null;

  function updateRow(id: string, field: "name" | "value", value: string) {
    setRows((current) =>
      current.map((row) => (row.id === id ? { ...row, [field]: value } : row)),
    );
    setSaveError(null);
  }

  function addRow() {
    setRows((current) => [
      ...current,
      {
        id: `new-${nextID.current++}`,
        name: "",
        value: "",
        source: "",
      },
    ]);
    setSaveError(null);
  }

  function removeRow(id: string) {
    setRows((current) => current.filter((row) => row.id !== id));
    setSaveError(null);
  }

  async function save() {
    if (validationError || !dirty || saving) return;
    setSaveError(null);
    const result = await onSave(
      rows.map(({ name, value }) => ({
        name: name.trim(),
        value: value.trim(),
      })),
    );
    if (result.ok) {
      setSavedRows(rows);
      return;
    }
    setSaveError(result.error ?? t("databases.insightsParamsSaveError"));
  }

  return (
    <div className="space-y-3">
      {rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {t("databases.insightsNoParams")}
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th className="pb-1 pr-3 font-medium">
                  {t("databases.insightsColParam")}
                </th>
                <th className="pb-1 pr-3 font-medium">
                  {t("databases.insightsColSetting")}
                </th>
                <th className="pb-1 pr-3 font-medium">
                  {t("databases.insightsColSource")}
                </th>
                <th className="w-9 pb-1">
                  <span className="sr-only">
                    {t("databases.insightsParamsActions")}
                  </span>
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row, index) => (
                <tr key={row.id} className="border-b last:border-0">
                  <td className="py-1.5 pr-3">
                    <Input
                      className="h-8 min-w-48 font-mono text-xs"
                      value={row.name}
                      onChange={(event) =>
                        updateRow(row.id, "name", event.target.value)
                      }
                      aria-label={t("databases.insightsParamNameLabel", {
                        index: index + 1,
                      })}
                    />
                  </td>
                  <td className="py-1.5 pr-3">
                    <Input
                      className="h-8 min-w-40 font-mono text-xs"
                      value={row.value}
                      onChange={(event) =>
                        updateRow(row.id, "value", event.target.value)
                      }
                      aria-label={t("databases.insightsParamValueLabel", {
                        index: index + 1,
                      })}
                    />
                  </td>
                  <td className="py-1.5 pr-3 text-muted-foreground">
                    {row.source || t("databases.insightsParamsPending")}
                  </td>
                  <td className="py-1.5 text-right">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      onClick={() => removeRow(row.id)}
                      aria-label={t("databases.insightsParamsRemove", {
                        name:
                          row.name ||
                          t("databases.insightsParamsUnnamed", {
                            index: index + 1,
                          }),
                      })}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {(validationError || saveError) && (
        <Alert variant="destructive" role="alert">
          <AlertDescription>{validationError ?? saveError}</AlertDescription>
        </Alert>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          <Plus className="h-3.5 w-3.5" />
          {t("databases.insightsParamsAdd")}
        </Button>
        <PermissionTooltip reason={permissionReason}>
          <Button
            type="button"
            size="sm"
            disabled={
              !dirty || Boolean(validationError) || saving || permissionBlocked
            }
            onClick={() => void save()}
          >
            {saving
              ? t("databases.insightsParamsSaving")
              : t("databases.insightsParamsSave")}
          </Button>
        </PermissionTooltip>
        {dirty && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={saving}
            onClick={() => {
              setRows(savedRows);
              setSaveError(null);
            }}
          >
            {t("databases.insightsParamsDiscard")}
          </Button>
        )}
      </div>
    </div>
  );
}
