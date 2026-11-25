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

import { useState } from "react";
import { Button } from "@/common/components/ui/button";
import { Card } from "@/common/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { isValidEmail } from "@/common/lib/utils/email";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useBillingEmail } from "../hooks/use-billing-email";
import {
  BillingEmailCardBody,
  BillingEmailCardHeader,
  BillingEmailCardSkeleton,
} from "./billing-email-card-skeleton";

/**
 * The workspace's billing email (w6/m627): where its invoices and billing
 * notices go. As on Render, a Pro or higher workspace sets an address of its
 * own, and a Hobby workspace's is its owner's account email, shown read-only
 * with the plan that unlocks it. Every member who may view billing details
 * reads it (Admin, Developer, Billing; w9/m620); only Admin and Billing
 * change it, so a Developer's Edit is held with the role's reason.
 */
export function BillingEmailCard() {
  const { currentWorkspaceId } = useWorkspace();
  return <BillingEmailCardView workspaceId={currentWorkspaceId ?? ""} />;
}

export function BillingEmailCardView({ workspaceId }: { workspaceId: string }) {
  const { t } = useTranslations();
  const { billingEmail, loading, saving, save } = useBillingEmail(workspaceId);
  const { denied: manageDenied, reason: manageReason } =
    useCapabilityGate("canManageBilling");
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const valid = isValidEmail(draft);

  if (!billingEmail && (loading || !workspaceId)) {
    return <BillingEmailCardSkeleton />;
  }

  function edit() {
    setDraft(billingEmail?.email ?? "");
    setOpen(true);
  }

  async function submit() {
    if (await save(draft.trim())) setOpen(false);
  }

  return (
    <Card>
      <BillingEmailCardHeader />
      {!billingEmail ? (
        // A failed read must not pass for an address.
        <BillingEmailCardBody
          address={
            <span className="font-normal text-destructive">
              {t("usage.billingEmailUnreadable")}
            </span>
          }
          note={null}
          action={null}
        />
      ) : (
        <BillingEmailCardBody
          address={
            billingEmail.email || (
              <span className="font-normal text-muted-foreground">
                {t("usage.billingEmailNone")}
              </span>
            )
          }
          note={
            billingEmail.configurable ? null : t("usage.billingEmailHobbyNote")
          }
          action={
            billingEmail.configurable ? (
              <PermissionTooltip reason={manageReason}>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={saving || manageDenied}
                  onClick={edit}
                >
                  {t("usage.billingEmailEdit")}
                </Button>
              </PermissionTooltip>
            ) : null
          }
        />
      )}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("usage.billingEmailDialogTitle")}</DialogTitle>
            <DialogDescription>
              {t("usage.billingEmailDialogDescription")}
            </DialogDescription>
          </DialogHeader>
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (valid && !saving) void submit();
            }}
          >
            <Label htmlFor="billing-email-address">
              {t("usage.billingEmailLabel")}
            </Label>
            <Input
              id="billing-email-address"
              type="email"
              autoComplete="email"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              aria-invalid={draft !== "" && !valid}
            />
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              disabled={!valid || saving}
              loading={saving}
              onClick={() => void submit()}
            >
              {t("usage.billingEmailSave")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
