import { Tabs, TabsList, TabsTrigger } from "@/common/components/ui/tabs";
import { useTranslations } from "@/common/hooks/use-translations";

/** Which keys the API keys panel lists: the caller's, or every member's. */
export type ApiKeysScope = "own" | "all";

/**
 * An Admin's switch between their own keys and every member's (w9/m539).
 * Every member's list is a bex addition for revoking a demoted member's keys
 * and keys with no recorded creator; Render shows each person their own.
 * The panel and its skeleton draw this one control, disabled while the role
 * is still unknown, so the two cannot drift apart.
 */
export function ApiKeysScopeTabs({
  value,
  onValueChange,
  disabled = false,
}: {
  value: ApiKeysScope;
  onValueChange?: (scope: ApiKeysScope) => void;
  disabled?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <Tabs
      value={value}
      onValueChange={(next) => onValueChange?.(next as ApiKeysScope)}
    >
      <TabsList aria-label={t("apiKeys.scopeLabel")}>
        {/* The table below is not a Tabs panel, so the tabs name none (an
            aria-controls to a missing id is invalid ARIA). */}
        <TabsTrigger value="own" disabled={disabled} aria-controls={undefined}>
          {t("apiKeys.scopeOwn")}
        </TabsTrigger>
        <TabsTrigger value="all" disabled={disabled} aria-controls={undefined}>
          {t("apiKeys.scopeAll")}
        </TabsTrigger>
      </TabsList>
    </Tabs>
  );
}
