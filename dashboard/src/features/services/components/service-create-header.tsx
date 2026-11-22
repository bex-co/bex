import { PageHeader } from "@/common/components/page-frame";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  serviceTypeCreateCopy,
  type ServiceType,
} from "@/features/services/lib/create-context";

/** A service create page's heading, ready and pending: the type's title and
 *  subtitle, and Render's note under the types with no public URL. */
export function ServiceCreateHeader({
  serviceType,
}: {
  serviceType: ServiceType;
}) {
  const { t } = useTranslations();
  const copy = serviceTypeCreateCopy(serviceType);
  const noPublicUrl =
    serviceType === "private_service" || serviceType === "background_worker";
  return (
    <PageHeader
      title={t(copy.titleKey)}
      description={
        <>
          <span className="block">{t(copy.descriptionKey)}</span>
          {noPublicUrl ? (
            <span className="block">{t("services.createNoPublicUrlNote")}</span>
          ) : null}
        </>
      }
    />
  );
}
