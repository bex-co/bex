import { Input } from "@/common/components/ui/input";
import { Switch } from "@/common/components/ui/switch";
import { Textarea } from "@/common/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/common/components/ui/form";
import { useTranslations } from "@/common/hooks/use-translations";
import { AGENT_OPTIONS } from "@/features/agent-sessions/lib/composer";
import type { AgentOption } from "@/features/agent-sessions/lib/composer";

export interface ConfigurationFieldsProps {
  /** The branch the composer would derive from the current task, shown while
   *  the field is empty — typing here overrides it. */
  branchPlaceholder: string;
}

/**
 * The Configuration popover body: agent/model/model endpoint/egress allowlist
 * plus the branch override. Reads the composer's form off `FormProvider`
 * context, so values and server-anchored errors round-trip without threading
 * the form instance down.
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
        name="agent"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("agentSessions.agentLabel")}</FormLabel>
            <Select
              value={field.value}
              onValueChange={(v) => field.onChange(v as AgentOption)}
            >
              <FormControl>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
              </FormControl>
              <SelectContent>
                {AGENT_OPTIONS.map((agent) => (
                  <SelectItem key={agent} value={agent}>
                    {t(`agentSessions.agent.${agent}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
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

      <FormField
        name="openPr"
        render={({ field }) => (
          <FormItem className="flex items-start justify-between gap-3">
            <div className="space-y-1">
              <FormLabel>{t("agentSessions.openPrLabel")}</FormLabel>
              <FormDescription>{t("agentSessions.openPrHint")}</FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={field.value}
                onCheckedChange={field.onChange}
                aria-label={t("agentSessions.openPrLabel")}
              />
            </FormControl>
          </FormItem>
        )}
      />
    </>
  );
}
