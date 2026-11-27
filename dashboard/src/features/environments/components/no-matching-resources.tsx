import { useTranslations } from "@/common/hooks/use-translations";

/** A search that matches none of a section's rows. */
export function NoMatchingResources() {
  const { t } = useTranslations();
  return (
    <div className="py-6 text-center" role="status">
      <p className="font-medium">{t("projects.noMatchesTitle")}</p>
      <p className="mt-1 text-sm text-muted-foreground">
        {t("projects.noMatchesBody")}
      </p>
    </div>
  );
}
