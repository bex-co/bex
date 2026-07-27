import { useMemo } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import {
  MAXMEMORY_POLICIES,
  RECOMMENDED_MAXMEMORY_POLICY,
} from "@/features/keyvalue/lib/labels";
import { useSetKeyValueMaxmemoryPolicy } from "@/features/keyvalue/hooks/use-set-key-value-maxmemory-policy";

/**
 * The Key Value detail's Maxmemory Policy section: change the eviction policy
 * after create (w7/007). The policy is already updatable on every programmatic
 * surface (REST PATCH / GraphQL setKeyValueMaxmemoryPolicy / MCP, w7/m45); this
 * closes the dashboard gap so it isn't a create-only field. Edit-in-place with
 * the shared select row (w5/m55): a disabled select + pencil that swaps for
 * Cancel / "Save changes", matching the services Settings page and Render.
 */
export function KeyValueMaxmemoryPolicySection({ id }: { id: string }) {
  const { t } = useTranslations();
  const { policy, loading, saving, save } = useSetKeyValueMaxmemoryPolicy(id);
  // Stable identity so EditableFieldRow's focus effect doesn't re-run each render.
  const options = useMemo(
    () =>
      MAXMEMORY_POLICIES.map((p) => ({
        value: p,
        label:
          p === RECOMMENDED_MAXMEMORY_POLICY
            ? `${p} ${t("keyvalue.fieldMaxmemoryRecommended")}`
            : p,
      })),
    [t],
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.maxmemoryTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.maxmemoryDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {loading && !policy ? (
          <Skeleton className="h-9 w-full" />
        ) : (
          <EditableFieldRow
            label={t("keyvalue.maxmemoryLabel")}
            hint={t("keyvalue.fieldMaxmemoryPolicyHint")}
            value={policy}
            editLabel={t("keyvalue.maxmemoryEdit")}
            busy={saving}
            options={options}
            onSave={save}
          />
        )}
      </CardContent>
    </Card>
  );
}
