import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  actionForBoolean,
  type CapabilityBoolean,
} from "@/features/capabilities/lib/capability-policy";

/**
 * Disable-with-reason for one capability (w9/m84): a control the caller's
 * role cannot use stays visible, held, and says why, instead of answering a
 * 403 on save. Every service and datastore setting is `canCreate` since
 * w9/m619, as Render's role table gives a Contributor no "Modify service
 * configuration"; lifecycle verbs (restart, suspend, failover, cron runs,
 * cache purge) stay on `canOperate`.
 *
 * `denied` is the rendered fail-closed flag, so a control is held while the
 * capability is still checking; `reason` is the role's sentence, undefined
 * until there is one to show.
 */
export function useCapabilityGate(key: CapabilityBoolean): {
  denied: boolean;
  reason: string | undefined;
} {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const reasonKey = capabilities.reasonKey(actionForBoolean(key));
  return {
    denied: !capabilities[key],
    reason: reasonKey ? t(reasonKey) : undefined,
  };
}
