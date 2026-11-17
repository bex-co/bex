import { AlertTriangle, SearchX } from "lucide-react";
import { Card, CardContent } from "@/common/components/ui/card";
import { PanelCenteredState } from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * An org page whose org could not be read: ORGANIZATION_NOT_FOUND (no such
 * org, or the caller is not in it; bex-api answers both the same) or a failed
 * read.
 */
export function OrganizationUnavailable({ notFound }: { notFound: boolean }) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardContent>
        {notFound ? (
          <PanelCenteredState
            icon={<SearchX />}
            title={t("organizations.notFoundTitle")}
            body={t("organizations.notFoundBody")}
          />
        ) : (
          <PanelCenteredState
            icon={<AlertTriangle />}
            title={t("organizations.errorTitle")}
            body={t("organizations.errorBody")}
          />
        )}
      </CardContent>
    </Card>
  );
}
