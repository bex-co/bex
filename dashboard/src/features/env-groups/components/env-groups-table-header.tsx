import { TableHead, TableHeader, TableRow } from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The Environment Groups list's five column labels, shared by the ready table
 * and its loading preview so both lay out the same columns (w4/m220). Labels
 * never wrap: in the horizontally scrolling mobile table, wrapping let the
 * count columns shrink to one character, stacking "Env Vars" vertically in a
 * 200px header.
 */
export function EnvGroupsTableHeader() {
  const { t } = useTranslations();
  return (
    <TableHeader>
      <TableRow>
        <TableHead className="whitespace-nowrap">
          {t("envGroups.colName")}
        </TableHead>
        <TableHead className="whitespace-nowrap">
          {t("envGroups.colEnvironment")}
        </TableHead>
        <TableHead className="whitespace-nowrap text-right">
          {t("envGroups.colEnvVars")}
        </TableHead>
        <TableHead className="whitespace-nowrap text-right">
          {t("envGroups.colSecretFiles")}
        </TableHead>
        <TableHead className="whitespace-nowrap text-right">
          {t("envGroups.colUpdated")}
        </TableHead>
      </TableRow>
    </TableHeader>
  );
}
