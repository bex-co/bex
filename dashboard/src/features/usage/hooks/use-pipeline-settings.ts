// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { useCallback, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  PipelineSettingsDocument,
  SetPipelineSpendLimitDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { money } from "@/features/usage/lib/charges";

export interface UsePipelineSettingsResult {
  /** The monthly pipeline spend limit ("25.00"), or null when there is none. */
  spendLimitUsd: string | null;
  loading: boolean;
  error: Error | undefined;
  saving: boolean;
  /** Sets the limit, or removes it with null; resolves true on success. */
  save: (spendLimitUsd: string | null) => Promise<boolean>;
}

/**
 * A workspace's Build Pipeline settings (w6/m309): its monthly spend limit on
 * build minutes beyond the plan's included ones.
 */
export function usePipelineSettings(
  workspaceId: string,
): UsePipelineSettingsResult {
  const { t, i18n } = useTranslations();
  const { data, loading, error } = useQuery(PipelineSettingsDocument, {
    variables: { workspaceId },
    errorPolicy: "all",
  });
  const [mutate] = useMutation(SetPipelineSpendLimitDocument);
  const [saving, setSaving] = useState(false);

  const save = useCallback(
    async (spendLimitUsd: string | null) => {
      setSaving(true);
      try {
        // The result normalizes into the query's cached PipelineSettings
        // (keyed by workspaceId), so the card updates without a refetch.
        const { data: result } = await mutate({
          variables: { workspaceId, spendLimitUsd },
        });
        const saved = result?.setPipelineSpendLimit?.spendLimitUsd;
        toast.success(
          saved
            ? t("usage.pipelineSpendLimitSaved", {
                limit: money(Number(saved), i18n.language),
              })
            : t("usage.pipelineSpendLimitRemoved"),
        );
        return true;
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("usage.pipelineSpendLimitError")),
        );
        return false;
      } finally {
        setSaving(false);
      }
    },
    [mutate, t, i18n.language, workspaceId],
  );

  return {
    spendLimitUsd: data?.pipelineSettings?.spendLimitUsd ?? null,
    loading,
    error,
    saving,
    save,
  };
}
