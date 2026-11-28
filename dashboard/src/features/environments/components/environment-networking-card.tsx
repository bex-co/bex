import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";
import { IPAllowListProxiedNotice } from "@/common/components/ip-allow-list-proxied-notice";
import { InboundIPLevels } from "@/common/components/inbound-ip-levels";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  ALLOW_ALL_PAIR_ENTRIES,
  ipAllowListEntryKey,
  type IPAllowListEntryDraft,
} from "@/common/lib/ip-allow-list";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { useUpdateEnvironment } from "@/features/environments/hooks/use-update-environment";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { UpgradePlanLink } from "@/features/workspaces/components/upgrade-plan-link";

/**
 * Render's Networking card on an environment's settings page (w1/m683): the
 * shared inbound IP rules editor over the environment's list, on Scale and
 * higher (ADR030). Below Scale the rules already set stay listed and
 * enforced, and the only write offered is resetting them to allow all. Above
 * them it shows the workspace's rules, which also apply to every resource in
 * the environment, as Render's does (w4/m710).
 */
export function EnvironmentNetworkingCard({
  environment,
  belowScale,
}: {
  environment: EnvironmentView;
  belowScale: boolean;
}) {
  const { t } = useTranslations();
  const { update, saving } = useUpdateEnvironment();
  const [draft, setDraft] = useState<IPAllowListEntryDraft[] | null>(null);

  async function save(entries: EnvironmentView["ipAllowListEntries"]) {
    const result = await update(environment.id, {
      ipAllowListEntries: entries.map((entry) => ({
        cidrBlock: entry.cidrBlock.trim(),
        description: entry.description.trim(),
      })),
    });
    if (result.ok) {
      toast.success(
        t("environments.ipAllowListSaved", { name: environment.name }),
      );
      return true;
    }
    toast.error(
      mutationErrorMessage(result.error, t("environments.settingsUpdateError")),
    );
    return false;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("environments.networkingCardTitle")}</CardTitle>
        <CardDescription>{t("environments.ipAllowListLabel")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <IPAllowListProxiedNotice
          domains={environment.ipAllowListProxiedDomains}
        />
        <InboundIPLevels
          saved={environment.ipAllowListEntries}
          draft={draft ?? environment.ipAllowListEntries}
          ownType={t("common.inboundIpLevelEnvironment")}
        />
        <IPAllowListEditor
          // The saved list remounts the draft: a save, or another Admin's.
          key={ipAllowListEntryKey(environment.ipAllowListEntries)}
          entries={environment.ipAllowListEntries}
          onDraftChange={setDraft}
          saving={saving}
          onSave={save}
          clearOnly={
            belowScale
              ? {
                  hint: (
                    <>
                      {t("environments.ipAllowListScalePlanOnly")}{" "}
                      <Button
                        asChild
                        variant="link"
                        className="h-auto p-0 text-xs"
                      >
                        <UpgradePlanLink plan="scale">
                          {t("environments.upgrade")}
                        </UpgradePlanLink>
                      </Button>
                    </>
                  ),
                  clearAll: t("environments.ipAllowListResetAllowAll"),
                }
              : undefined
          }
          emptyBlocks={{
            open: t("environments.ipAllowListAllowsAll"),
            allowAll: ALLOW_ALL_PAIR_ENTRIES,
          }}
          labels={{
            hint: t("environments.ipAllowListHint"),
            empty: t("environments.ipAllowListOpen"),
            descriptionPlaceholder: t(
              "environments.ipAllowListDescriptionPlaceholder",
            ),
            cidr: t("environments.ipAllowListNewCIDR"),
            cidrRule: (number) =>
              t("environments.ipAllowListRuleCIDR", { number }),
            description: t("environments.ipAllowListNewDescription"),
            descriptionRule: (number) =>
              t("environments.ipAllowListRuleDescription", { number }),
            add: t("environments.ipAllowListAdd"),
            save: t("environments.settingsCardSave"),
            cancel: t("environments.cancel"),
            invalid: t("environments.ipAllowListInvalid"),
            remove: (cidr) => t("environments.ipAllowListRemove", { cidr }),
            moveUp: (cidr) => t("environments.ipAllowListMoveUp", { cidr }),
            moveDown: (cidr) => t("environments.ipAllowListMoveDown", { cidr }),
          }}
        />
      </CardContent>
    </Card>
  );
}
