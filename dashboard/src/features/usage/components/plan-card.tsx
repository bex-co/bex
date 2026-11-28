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

import { Link } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { OrganizationPlanNote } from "@/features/workspaces/components/organization-plan-note";
import {
  WORKSPACE_PLAN_CATALOG,
  organizationPlanOwner,
} from "@/features/workspaces/types";
import { AFTER_PLAN_UPDATE_BILLING } from "@/features/workspaces/lib/update-plan";

/**
 * The workspace's plan, first on the billing page — what everything below is
 * priced against. Change plan opens Render's plan page, `/billing/update-plan`
 * (w3/m708), which comes back here.
 *
 * A secondary workspace of an organization (w7/m464) shows its plan as the
 * org's and offers no change: Render's "Your primary workspace manages the
 * billing for the organization", so its note links to the primary's Billing
 * page when the caller is in it.
 */
export function PlanCard() {
  const { t } = useTranslations();
  const { currentWorkspace, workspaces, loading } = useWorkspace();
  const planOwner = organizationPlanOwner(currentWorkspace, workspaces);

  const entry = WORKSPACE_PLAN_CATALOG.find(
    (p) => p.id === currentWorkspace?.plan,
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.planTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {!currentWorkspace && loading ? (
          <Skeleton className="h-14 w-full" />
        ) : !currentWorkspace ? (
          <p className="text-sm text-muted-foreground">
            {t("workspaces.settingsEmpty")}
          </p>
        ) : (
          <div className="flex flex-wrap items-start justify-between gap-4 rounded-md border p-4">
            <div className="min-w-0">
              <p className="font-medium">
                {entry ? t(entry.nameKey) : currentWorkspace.plan}
              </p>
              {entry && (
                <p className="text-sm text-muted-foreground">
                  {t(entry.descriptionKey)}
                </p>
              )}
              {planOwner ? (
                <OrganizationPlanNote owner={planOwner} target="billing" />
              ) : null}
            </div>
            {planOwner ? null : (
              <Button asChild variant="outline" size="sm">
                <Link
                  to="/billing/update-plan"
                  state={AFTER_PLAN_UPDATE_BILLING}
                >
                  {t("usage.planChange")}
                </Link>
              </Button>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
