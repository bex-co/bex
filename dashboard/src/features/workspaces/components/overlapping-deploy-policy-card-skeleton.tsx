import type { ComponentProps } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { ReservedText } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { SettingRadioOption } from "@/features/workspaces/components/setting-radio-option";
import {
  OVERLAPPING_DEPLOY_POLICIES,
  type OverlappingDeployPolicy,
} from "@/features/workspaces/hooks/use-overlapping-deploy-policy";

/** The card's static header, shared by the ready card and its skeleton. */
export function OverlappingDeployPolicyCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("workspaces.overlappingDeployPolicyTitle")}</CardTitle>
      <CardDescription>
        {t("workspaces.overlappingDeployPolicyDescription")}
      </CardDescription>
    </CardHeader>
  );
}

const OPTION_KEYS: Record<
  OverlappingDeployPolicy,
  { label: string; description: string }
> = {
  wait: {
    label: "workspaces.overlappingDeployPolicyWait",
    description: "workspaces.overlappingDeployPolicyWaitDescription",
  },
  override: {
    label: "workspaces.overlappingDeployPolicyOverride",
    description: "workspaces.overlappingDeployPolicyOverrideDescription",
  },
};

/** One policy as a card radio: Render's label and sentence. */
export function OverlappingDeployPolicyOption({
  policy,
  ...props
}: {
  policy: OverlappingDeployPolicy;
} & Omit<ComponentProps<typeof SettingRadioOption>, "label" | "description">) {
  const { t } = useTranslations();
  const keys = OPTION_KEYS[policy];
  return (
    <SettingRadioOption
      {...props}
      label={t(keys.label)}
      description={t(keys.description)}
    />
  );
}

/**
 * OverlappingDeployPolicyCard's pending shape (w4/653): the static header, both
 * policy options, and the Save button an Admin sees (size sm), or the
 * Admin-only note a member reads in its place, as its own text so it wraps as
 * the ready one does (w7/094).
 */
export function OverlappingDeployPolicyCardSkeleton({
  canManage = true,
}: {
  canManage?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <Card data-testid="overlapping-deploy-policy-skeleton">
      <OverlappingDeployPolicyCardHeader />
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 gap-2">
          {OVERLAPPING_DEPLOY_POLICIES.map((policy) => (
            <OverlappingDeployPolicyOption
              key={policy}
              policy={policy}
              checked={false}
              pending
              disabled
              tabIndex={-1}
            />
          ))}
        </div>
        {canManage ? (
          <Skeleton className="h-8 w-16" />
        ) : (
          <p className="relative w-fit text-sm" aria-hidden>
            <ReservedText>
              {t("workspaces.overlappingDeployPolicyAdminOnly")}
            </ReservedText>
          </p>
        )}
      </CardContent>
    </Card>
  );
}
