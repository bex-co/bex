import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  MAXMEMORY_POLICIES,
  RECOMMENDED_MAXMEMORY_POLICY,
} from "@/features/keyvalue/lib/labels";
import { useSetKeyValueMaxmemoryPolicy } from "@/features/keyvalue/hooks/use-set-key-value-maxmemory-policy";

/**
 * The Key Value detail's Maxmemory Policy section: change the eviction policy
 * after create (w7/007). The policy is already updatable on every programmatic
 * surface (REST PATCH / GraphQL setKeyValueMaxmemoryPolicy / MCP, w7/m45); this
 * closes the dashboard gap so it isn't a create-only field. Structurally mirrors
 * KeyValuePlanSection (a Card with one control + Save) and shares the create
 * wizard's policy ladder so both offer the same options.
 */
export function KeyValueMaxmemoryPolicySection({ id }: { id: string }) {
  const { t } = useTranslations();
  const { policy, loading, saving, save } = useSetKeyValueMaxmemoryPolicy(id);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.maxmemoryTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.maxmemoryDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading && !policy ? (
          <Skeleton className="h-9 w-full" />
        ) : (
          /* key reseeds the draft from the server value whenever it changes
             (e.g. after a save), avoiding an effect-based state sync — the
             networking panel's pattern. */
          <MaxmemoryPolicyEditor
            key={policy}
            current={policy}
            saving={saving}
            onSave={save}
          />
        )}
      </CardContent>
    </Card>
  );
}

function MaxmemoryPolicyEditor({
  current,
  saving,
  onSave,
}: {
  current: string;
  saving: boolean;
  onSave: (policy: string) => Promise<boolean>;
}) {
  const { t } = useTranslations();
  const [selected, setSelected] = useState<string>(current);
  const canSave = selected !== current && selected !== "" && !saving;

  return (
    <>
      <Select value={selected} onValueChange={setSelected}>
        <SelectTrigger
          id="kv-maxmemory-policy"
          className="w-full"
          aria-label={t("keyvalue.maxmemoryLabel")}
        >
          <SelectValue placeholder={t("keyvalue.maxmemoryPlaceholder")} />
        </SelectTrigger>
        <SelectContent>
          {MAXMEMORY_POLICIES.map((p) => (
            <SelectItem key={p} value={p}>
              {p}
              {p === RECOMMENDED_MAXMEMORY_POLICY
                ? ` ${t("keyvalue.fieldMaxmemoryRecommended")}`
                : ""}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className="text-sm text-muted-foreground">
        {t("keyvalue.fieldMaxmemoryPolicyHint")}
      </p>

      <div className="flex justify-end gap-2 border-t pt-4">
        <Button
          variant="outline"
          onClick={() => setSelected(current)}
          disabled={selected === current || saving}
        >
          {t("keyvalue.maxmemoryCancel")}
        </Button>
        <Button disabled={!canSave} onClick={() => void onSave(selected)}>
          {t("keyvalue.maxmemorySave")}
        </Button>
      </div>
    </>
  );
}
