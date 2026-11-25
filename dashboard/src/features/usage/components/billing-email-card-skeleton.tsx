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

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/** The card's static header, shared by the ready card and its skeleton. */
export function BillingEmailCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("usage.billingEmailTitle")}</CardTitle>
      <CardDescription>{t("usage.billingEmailDescription")}</CardDescription>
    </CardHeader>
  );
}

/**
 * BillingEmailCard's pending shape (w6/m627). The header is static copy, so
 * it renders as the ready card's text. Below it, the address line, then what
 * the plan decides: on Hobby the "Pro and above" note in its own copy, laid
 * out invisibly so it wraps as the ready note does; on a higher plan the Edit
 * button. The plan comes from the workspace list the shell already holds;
 * until it arrives, Hobby, the plan a new workspace starts on, stands in.
 *
 * Skeleton-only, so the route's pending component can import it without
 * pulling the page's chunk up front.
 */
export function BillingEmailCardSkeleton() {
  const { t } = useTranslations();
  const { currentWorkspace } = useWorkspace();
  const hobby = (currentWorkspace?.plan ?? "hobby") === "hobby";
  return (
    <Card data-testid="billing-email-skeleton">
      <BillingEmailCardHeader />
      <BillingEmailCardBody
        address={<Skeleton className="h-5 w-56 max-w-full" />}
        note={
          hobby ? (
            <span className="relative block">
              <ReservedText>{t("usage.billingEmailHobbyNote")}</ReservedText>
            </span>
          ) : null
        }
        action={
          hobby ? null : (
            <ReservedButton variant="outline" size="sm">
              {t("usage.billingEmailEdit")}
            </ReservedButton>
          )
        }
      />
    </Card>
  );
}

/**
 * The card's body layout, shared by the ready card and its skeleton so the
 * two cannot drift: the address, the plan's note under it, and the action
 * beside them, wrapping under them on a phone.
 */
export function BillingEmailCardBody({
  address,
  note,
  action,
}: {
  address: React.ReactNode;
  note: React.ReactNode;
  action: React.ReactNode;
}) {
  return (
    <CardContent className="flex flex-wrap items-center justify-between gap-3">
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex min-h-5 items-center text-sm font-medium break-all">
          {address}
        </div>
        {note ? (
          <div className="text-xs text-muted-foreground">{note}</div>
        ) : null}
      </div>
      {action}
    </CardContent>
  );
}
