import type { ButtonHTMLAttributes, Ref } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
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

/**
 * One policy as a card radio, Render's label and sentence. The skeleton renders
 * the same button unchecked and disabled, with its text transparent on a
 * pulsing bar, so it wraps alike at every width and language.
 */
export function OverlappingDeployPolicyOption({
  policy,
  checked,
  pending = false,
  buttonRef,
  ...props
}: {
  policy: OverlappingDeployPolicy;
  checked: boolean;
  pending?: boolean;
  buttonRef?: Ref<HTMLButtonElement>;
} & ButtonHTMLAttributes<HTMLButtonElement>) {
  const { t } = useTranslations();
  const keys = OPTION_KEYS[policy];
  const text = pending
    ? "bg-accent animate-pulse rounded-md box-decoration-clone text-transparent motion-reduce:animate-none"
    : "";
  return (
    <button
      {...props}
      ref={buttonRef}
      type="button"
      role="radio"
      aria-checked={checked}
      className={cn(
        "flex items-start gap-3 rounded-lg border p-3 text-left transition-colors disabled:cursor-not-allowed",
        !pending && "disabled:opacity-60",
        checked
          ? "border-primary ring-1 ring-primary"
          : "border-border enabled:hover:border-muted-foreground/50",
      )}
    >
      <span
        aria-hidden
        className={cn(
          "mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-full border",
          checked ? "border-primary" : "border-muted-foreground/50",
        )}
      >
        {checked ? <span className="bg-primary size-2 rounded-full" /> : null}
      </span>
      <span>
        <span className="block text-sm font-medium">
          <span className={text}>{t(keys.label)}</span>
        </span>
        <span className="text-muted-foreground block text-xs">
          <span className={text}>{t(keys.description)}</span>
        </span>
      </span>
    </button>
  );
}

/**
 * OverlappingDeployPolicyCard's pending shape (w4/653): the static header, both
 * policy options, and the Save button an Admin sees (size sm), or the one-line
 * Admin-only note a member reads in its place.
 */
export function OverlappingDeployPolicyCardSkeleton({
  canManage = true,
}: {
  canManage?: boolean;
}) {
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
          <Skeleton className="h-5 w-56" />
        )}
      </CardContent>
    </Card>
  );
}
