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
  SetWorkspaceBillingEmailDocument,
  WorkspaceBillingEmailDocument,
  type WorkspaceBillingEmailQuery,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  mutationErrorMessage,
  planFeatureRequired,
} from "@/common/lib/graphql-error";

export type BillingEmail = NonNullable<
  WorkspaceBillingEmailQuery["workspaceBillingEmail"]
>;

export interface UseBillingEmailResult {
  /** The billing email in effect, and whether the plan lets it change. */
  billingEmail: BillingEmail | null;
  loading: boolean;
  error: Error | undefined;
  saving: boolean;
  /** Sets the billing email; resolves true on success. */
  save: (email: string) => Promise<boolean>;
}

/**
 * A workspace's billing email (w6/m627): where its invoices and billing
 * notices go. Pro and higher set their own; a Hobby workspace's is its
 * owner's account email.
 */
export function useBillingEmail(workspaceId: string): UseBillingEmailResult {
  const { t } = useTranslations();
  const { data, loading, error } = useQuery(WorkspaceBillingEmailDocument, {
    variables: { workspaceId },
    skip: !workspaceId,
    errorPolicy: "all",
  });
  const [mutate] = useMutation(SetWorkspaceBillingEmailDocument);
  const [saving, setSaving] = useState(false);

  const save = useCallback(
    async (email: string) => {
      setSaving(true);
      try {
        // The result normalizes into the query's cached WorkspaceBillingEmail
        // (keyed by workspaceId), so the card updates without a refetch.
        const { data: result } = await mutate({
          variables: { workspaceId, email },
        });
        toast.success(
          t("usage.billingEmailSaved", {
            email: result?.setWorkspaceBillingEmail?.email ?? email,
          }),
        );
        return true;
      } catch (err) {
        // A plan refusal (the workspace moved to Hobby since the page
        // loaded) reads in the member's language, decided by its code.
        toast.error(
          planFeatureRequired(err, "billingEmail")
            ? t("usage.billingEmailHobbyNote")
            : mutationErrorMessage(err, t("usage.billingEmailError")),
        );
        return false;
      } finally {
        setSaving(false);
      }
    },
    [mutate, t, workspaceId],
  );

  return {
    billingEmail: data?.workspaceBillingEmail ?? null,
    loading,
    error,
    saving,
    save,
  };
}
