import { RadioDot } from "@/common/components/radio-dot";
import { useState } from "react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import { cn } from "@/common/lib/utils/utils";
import { useEdgeCache } from "@/features/services/hooks/use-edge-cache";

/** Render's `serviceDetails.cache.profile` values for a web service. */
const CACHE_PROFILES = [
  "no-cache",
  "origin-controlled",
  "origin-controlled-all",
] as const;
type CacheProfile = (typeof CACHE_PROFILES)[number];

const PROFILE_OPTIONS: Record<
  CacheProfile,
  { labelKey: string; descKey: string }
> = {
  "no-cache": {
    labelKey: "services.edgeCachingOptionNone",
    descKey: "services.edgeCachingOptionNoneDesc",
  },
  "origin-controlled": {
    labelKey: "services.edgeCachingOptionStatic",
    descKey: "services.edgeCachingOptionStaticDesc",
  },
  "origin-controlled-all": {
    labelKey: "services.edgeCachingOptionAll",
    descKey: "services.edgeCachingOptionAllDesc",
  },
};

function toCacheProfile(value: string | null | undefined): CacheProfile {
  return CACHE_PROFILES.find((p) => p === value) ?? "no-cache";
}

export interface EdgeCachingSectionProps {
  serviceId: string;
  /** Render offers edge caching only on paid web-service plans. */
  plan?: string | null;
  /** The saved profile; null/empty reads as "no-cache". */
  cacheProfile: string | null | undefined;
}

/**
 * Settings section for Render's Edge Caching: which responses the edge caches
 * (none, common static files, or every eligible response), saved explicitly,
 * plus a confirmed purge while caching is on. web_service only; the Settings
 * page gates rendering by type, and a free plan sees the section disabled.
 */
export function EdgeCachingSection({
  serviceId,
  plan,
  cacheProfile,
}: EdgeCachingSectionProps) {
  const { t } = useTranslations();
  const { setCacheProfile, purgeCache, saving, purging } = useEdgeCache();
  // The cache profile is a setting; purging the cache is an operation a
  // Contributor keeps (can_operate).
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");
  const { denied: operateDenied, reason: operateReason } =
    useCapabilityGate("canOperate");
  const saved = toCacheProfile(cacheProfile);
  const eligible = plan !== "free";
  // null follows the saved profile, so a save (or a refetch) re-syncs it.
  const [draft, setDraft] = useState<CacheProfile | null>(null);
  const selected = draft ?? saved;
  const dirty = selected !== saved;
  const [confirmPurge, setConfirmPurge] = useState(false);
  const locked = !eligible || saving || createDenied;
  const radio = useCardRadioGroup(CACHE_PROFILES, selected, setDraft, locked);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.edgeCachingTitle")}</CardTitle>
        <CardDescription>
          {t("services.edgeCachingDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <div id="edge-caching-file-types" className="text-sm font-medium">
            {t("services.edgeCachingFileTypesLabel")}
          </div>
          <div
            role="radiogroup"
            aria-labelledby="edge-caching-file-types"
            aria-disabled={!eligible || undefined}
            className="grid grid-cols-1 gap-2"
          >
            {CACHE_PROFILES.map((profile) => {
              const option = PROFILE_OPTIONS[profile];
              const checked = profile === selected;
              return (
                <button
                  key={profile}
                  {...radio(profile)}
                  type="button"
                  role="radio"
                  aria-checked={checked}
                  disabled={locked}
                  onClick={() => setDraft(profile)}
                  className={cn(
                    "flex items-start gap-3 rounded-lg border p-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-60",
                    checked
                      ? "border-primary ring-1 ring-primary"
                      : "border-border enabled:hover:border-muted-foreground/50",
                  )}
                >
                  <RadioDot checked={checked} className="mt-0.5" />
                  <span>
                    <span className="block text-sm font-medium">
                      {t(option.labelKey)}
                    </span>
                    <span className="text-muted-foreground block text-xs">
                      {t(option.descKey)}
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
          <p className="text-muted-foreground text-sm">
            {eligible
              ? t("services.edgeCachingPurgeOnDeploy")
              : t("services.edgeCachingPaidOnly")}
          </p>
          {/* The plan line already explains a free service's locked picker. */}
          {eligible && createReason ? (
            <p className="text-muted-foreground text-sm">{createReason}</p>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <PermissionTooltip reason={eligible ? createReason : undefined}>
            <Button
              size="sm"
              disabled={!eligible || !dirty || saving || createDenied}
              onClick={async () => {
                if (await setCacheProfile(serviceId, selected)) setDraft(null);
              }}
            >
              {saving
                ? t("services.edgeCachingSaving")
                : t("services.edgeCachingSave")}
            </Button>
          </PermissionTooltip>
          <PermissionTooltip reason={eligible ? operateReason : undefined}>
            <Button
              size="sm"
              variant="outline"
              disabled={
                !eligible || saved === "no-cache" || purging || operateDenied
              }
              onClick={() => setConfirmPurge(true)}
            >
              {t("services.edgeCachingPurgeAction")}
            </Button>
          </PermissionTooltip>
        </div>
      </CardContent>

      <ConfirmDialog
        open={confirmPurge}
        onOpenChange={setConfirmPurge}
        title={t("services.edgeCachingPurgeConfirmTitle")}
        description={t("services.edgeCachingPurgeConfirmBody")}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.edgeCachingPurgeAction")}
        pending={purging}
        onConfirm={() => {
          void purgeCache(serviceId);
          setConfirmPurge(false);
        }}
      />
    </Card>
  );
}
