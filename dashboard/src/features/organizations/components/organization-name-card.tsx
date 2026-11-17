import { useId, useState } from "react";
import { Loader2 } from "lucide-react";
import { Card, CardContent } from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { LabelError } from "@/common/components/label-error";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  MAX_LABEL_LENGTH,
  isInvalidLabel,
  isValidLabel,
} from "@/common/lib/utils/resource-label";
import { useRenameOrganization } from "@/features/organizations/hooks/use-rename-organization";
import {
  NAME_ROW_CLASS,
  OrganizationNameCardHeader,
} from "@/features/organizations/components/organization-settings-frame";

/**
 * Org Settings → name (ADR115: an Owner renames the org, `ChangeOrgNameEvent`).
 * Everyone in the org reads it here; only an Owner may change it.
 */
export function OrganizationNameCard({
  organizationId,
  name,
  canRename,
}: {
  organizationId: string;
  name: string;
  canRename: boolean;
}) {
  const { t } = useTranslations();
  const inputId = useId();
  const messageId = useId();
  const { rename, saving } = useRenameOrganization(organizationId);
  // null follows the saved name, so a rename (or a refetch) re-syncs it.
  const [draft, setDraft] = useState<string | null>(null);
  const value = draft ?? name;
  const invalid = isInvalidLabel(value);
  const unchanged = value.trim() === name;

  async function submit() {
    if (!canRename || saving || unchanged || !isValidLabel(value)) return;
    if (await rename(value.trim())) setDraft(null);
  }

  return (
    <Card>
      <OrganizationNameCardHeader />
      <CardContent className="space-y-2">
        <form
          className={NAME_ROW_CLASS}
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div className="min-w-0 flex-1 space-y-2">
            <Label htmlFor={inputId}>{t("organizations.fieldName")}</Label>
            <Input
              id={inputId}
              value={value}
              maxLength={MAX_LABEL_LENGTH}
              disabled={!canRename}
              aria-invalid={invalid || undefined}
              aria-describedby={invalid || !canRename ? messageId : undefined}
              onChange={(event) => setDraft(event.target.value)}
            />
          </div>
          {canRename ? (
            <Button
              type="submit"
              disabled={saving || unchanged || !isValidLabel(value)}
            >
              {saving ? <Loader2 className="animate-spin" /> : null}
              {t("organizations.nameSave")}
            </Button>
          ) : null}
        </form>
        {invalid ? (
          <LabelError id={messageId} value={value} />
        ) : !canRename ? (
          <p id={messageId} className="text-xs text-muted-foreground">
            {t("organizations.nameOwnerOnly")}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
