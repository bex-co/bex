import { Input } from "@/common/components/ui/input";
import { Textarea } from "@/common/components/ui/textarea";
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/common/components/ui/form";
import { useTranslations } from "@/common/hooks/use-translations";

export interface ConfigurationFieldsProps {
  /** The branch the composer would derive from the current task, shown while
   *  the field is empty — typing here overrides it. */
  branchPlaceholder: string;
}

/**
 * The Advanced popover body: branch override, model, model endpoint, and
 * egress allowlist. The agent select lives on the composer toolbar. Reads the
 * composer's form off `FormProvider` context so values and server-anchored
 * errors round-trip without threading the form instance down.
 */
export function ConfigurationFields({
  branchPlaceholder,
}: ConfigurationFieldsProps) {
  const { t } = useTranslations();
  return (
    <>
      <FormField
        name="branch"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.branchLabel")}</FormLabel>
            <FormControl>
              <Input
                {...field}
                autoComplete="off"
                placeholder={branchPlaceholder}
              />
            </FormControl>
            <FormDescription>{t("agentSessions.branchHint")}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        name="model"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.modelLabel")}</FormLabel>
            <FormControl>
              <Input
                {...field}
                autoComplete="off"
                placeholder={t("agentSessions.modelPlaceholder")}
              />
            </FormControl>
            <FormDescription>{t("agentSessions.modelHint")}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        name="modelEndpoint"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.modelEndpointLabel")}</FormLabel>
            <FormControl>
              <Input
                {...field}
                autoComplete="off"
                inputMode="url"
                placeholder={t("agentSessions.modelEndpointPlaceholder")}
              />
            </FormControl>
            <FormDescription>
              {t("agentSessions.modelEndpointHint")}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        name="egress"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.egressLabel")}</FormLabel>
            <FormControl>
              <Textarea
                {...field}
                rows={3}
                className="font-mono text-sm"
                placeholder={t("agentSessions.egressPlaceholder")}
              />
            </FormControl>
            <FormDescription>{t("agentSessions.egressHint")}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </>
  );
}
