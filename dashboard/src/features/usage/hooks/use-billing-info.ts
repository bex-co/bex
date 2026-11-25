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
  SetWorkspaceBillingInfoDocument,
  WorkspaceBillingInfoDocument,
  type BillingAddressInput,
  type BillingTaxIdInput,
  type WorkspaceBillingInfoQuery,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  graphQLErrorExtensions,
  hasGraphQLErrorCode,
  mutationErrorMessage,
  relayableReason,
} from "@/common/lib/graphql-error";

export type BillingInfo = NonNullable<
  WorkspaceBillingInfoQuery["workspaceBillingInfo"]
>;

/** The fields a BILLING_INFO_INVALID refusal may name (its params.field). */
export const BILLING_INFO_FIELDS = [
  "companyName",
  "address.line1",
  "address.line2",
  "address.city",
  "address.state",
  "address.postalCode",
  "address.country",
  "taxId.type",
  "taxId.value",
] as const;
export type BillingInfoField = (typeof BILLING_INFO_FIELDS)[number];

/**
 * The whole billing info a save writes: it replaces all three parts. An
 * empty company name bills the workspace's name again; a null address or tax
 * ID removes it.
 */
export interface BillingInfoInput {
  companyName: string;
  address: BillingAddressInput | null;
  taxId: BillingTaxIdInput | null;
}

/** A save's outcome: a refused field comes back for the form to show. */
export type SaveBillingInfoResult =
  | { ok: true }
  | { ok: false; fieldError: { field: BillingInfoField; message: string } }
  | { ok: false; fieldError: null };

export interface UseBillingInfoResult {
  billingInfo: BillingInfo | null;
  loading: boolean;
  error: Error | undefined;
  saving: boolean;
  save: (input: BillingInfoInput) => Promise<SaveBillingInfoResult>;
}

/**
 * The field bex-api refused billing info for, with its reason
 * (`BILLING_INFO_INVALID`'s params.field and params.reason), or null for any
 * other error.
 */
export function billingInfoFieldError(
  err: unknown,
): { field: BillingInfoField; message: string } | null {
  const extensions = graphQLErrorExtensions(err, "BILLING_INFO_INVALID");
  const field = BILLING_INFO_FIELDS.find(
    (known) => known === extensions?.["field"],
  );
  if (!field) return null;
  const reason = extensions?.["reason"];
  const message =
    typeof reason === "string" && reason ? reason : relayableReason(err);
  return {
    field,
    message: message.charAt(0).toUpperCase() + message.slice(1),
  };
}

/**
 * A workspace's billing info (w6/m628): the company name, address and VAT or
 * tax ID its invoices carry, as on Render's Billing page.
 */
export function useBillingInfo(workspaceId: string): UseBillingInfoResult {
  const { t } = useTranslations();
  const { data, loading, error } = useQuery(WorkspaceBillingInfoDocument, {
    variables: { workspaceId },
    skip: !workspaceId,
    errorPolicy: "all",
  });
  const [mutate] = useMutation(SetWorkspaceBillingInfoDocument);
  const [saving, setSaving] = useState(false);

  const save = useCallback(
    async (input: BillingInfoInput): Promise<SaveBillingInfoResult> => {
      setSaving(true);
      try {
        // The result normalizes into the query's cached WorkspaceBillingInfo
        // (keyed by workspaceId), so the card updates without a refetch.
        const { data: result } = await mutate({
          variables: { workspaceId, ...input },
        });
        toast.success(
          t("usage.billingInfoSaved", {
            name: result?.setWorkspaceBillingInfo?.billTo ?? "",
          }),
        );
        return { ok: true };
      } catch (err) {
        // A refused field is shown under that field, not in a toast.
        const fieldError = billingInfoFieldError(err);
        if (fieldError) return { ok: false, fieldError };
        toast.error(
          hasGraphQLErrorCode(err, "BILLING_CUSTOMER_NOT_READY")
            ? t("usage.billingInfoCustomerNotReady")
            : mutationErrorMessage(err, t("usage.billingInfoError")),
        );
        return { ok: false, fieldError: null };
      } finally {
        setSaving(false);
      }
    },
    [mutate, t, workspaceId],
  );

  return {
    billingInfo: data?.workspaceBillingInfo ?? null,
    loading,
    error,
    saving,
    save,
  };
}
