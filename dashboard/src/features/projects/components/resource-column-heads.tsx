import { TableHead } from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The resource table's columns after Name (`ResourceTable`): Type, Status and
 * Created, or Render's operational metadata (`projectMetadata`): Status,
 * Runtime, Region and Updated. Shared with the pending frames that preview
 * the table (a Blueprint's Resources, w3/m680), so both lay out the same
 * columns at every width.
 */
export function ResourceColumnHeads({
  projectMetadata,
}: {
  projectMetadata: boolean;
}) {
  const { t } = useTranslations();
  return (
    <>
      {!projectMetadata ? (
        <TableHead className="hidden sm:table-cell">
          {t("projects.colType")}
        </TableHead>
      ) : null}
      <TableHead>{t("projects.colStatus")}</TableHead>
      {projectMetadata ? (
        <>
          <TableHead className="hidden md:table-cell">
            {t("projects.colRuntime")}
          </TableHead>
          <TableHead className="hidden lg:table-cell">
            {t("projects.colRegion")}
          </TableHead>
          <TableHead className="hidden sm:table-cell">
            {t("projects.colUpdated")}
          </TableHead>
        </>
      ) : (
        <TableHead className="hidden md:table-cell">
          {t("projects.colCreated")}
        </TableHead>
      )}
    </>
  );
}
