import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { RelativeAge } from "@/common/components/relative-time";
import { ValidatePanel } from "@/features/blueprints/components/validate-panel";
import { BlueprintSettingsCard } from "@/features/blueprints/components/blueprint-settings-card";
import { DisconnectBlueprint } from "@/features/blueprints/components/disconnect-blueprint";
import { useBlueprintDetail } from "@/features/blueprints/lib/blueprint-detail";

/**
 * A Blueprint's Settings page, Render's `/blueprint/<id>/settings`: Render's
 * Settings card (w3/m679), which heads the page, then bex's extras, which
 * Render's page has no counterpart for (the repository and dates, the
 * render.yaml manifest and Validate), then Disconnect Blueprint last, where
 * Render puts it.
 */
export function BlueprintSettingsPage() {
  const { t } = useTranslations();
  const { blueprint } = useBlueprintDetail();

  return (
    <>
      <BlueprintSettingsCard blueprint={blueprint} />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">
            {t("blueprints.detailsTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm sm:grid-cols-3">
            <div className="col-span-2 min-w-0 sm:col-span-1">
              <dt className="text-muted-foreground">
                {t("blueprints.metaRepo")}
              </dt>
              <dd className="truncate font-medium" title={blueprint.repo}>
                {blueprint.repo}
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">
                {t("blueprints.metaCreated")}
              </dt>
              <dd className="font-medium">
                <RelativeAge value={blueprint.createdAt} />
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">
                {t("blueprints.metaUpdated")}
              </dt>
              <dd className="font-medium">
                <RelativeAge value={blueprint.lastSync} />
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">
            {t("blueprints.manifestTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <pre className="overflow-x-auto rounded-md bg-muted p-4 text-xs leading-relaxed">
            <code>{blueprint.manifest || "—"}</code>
          </pre>
        </CardContent>
      </Card>

      <ValidatePanel manifest={blueprint.manifest} blueprintId={blueprint.id} />

      <div>
        <DisconnectBlueprint blueprintId={blueprint.id} name={blueprint.name} />
      </div>
    </>
  );
}
