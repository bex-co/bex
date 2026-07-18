import { useState } from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServiceInstances } from "@/features/services/hooks/use-service-instances";
import { WebShellTerminal } from "@/features/services/components/web-shell-terminal";

// ANY_INSTANCE is the sentinel for "let the gateway pick a random Ready replica"
// (the empty-instance case native SSH also uses). Radix Select can't hold an
// empty-string value, so the sentinel maps to `undefined` on the way out.
const ANY_INSTANCE = "any";

/**
 * The Web Shell panel: Render's "Select an instance" picker above the in-browser
 * terminal (w2/m55). Choosing an instance targets that specific replica;
 * "Any ready instance" lets the gateway pick one, matching native SSH. Changing
 * the selection re-opens the terminal against the new target.
 */
export function WebShellPanel({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { instances } = useServiceInstances(serviceId);
  const [selected, setSelected] = useState<string>(ANY_INSTANCE);

  // A selection that no longer exists (an instance was replaced by a deploy)
  // falls back to "any" so the terminal never targets a vanished pod.
  const known = selected === ANY_INSTANCE || instances.some((i) => i.id === selected);
  const effective = known ? selected : ANY_INSTANCE;
  const instanceId = effective === ANY_INSTANCE ? undefined : effective;

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground text-sm">
          {t("services.shellInstanceLabel")}
        </span>
        <Select
          value={effective}
          onValueChange={setSelected}
          disabled={instances.length === 0}
        >
          <SelectTrigger size="sm" className="w-[220px]">
            <SelectValue placeholder={t("services.shellInstanceSelect")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY_INSTANCE}>
              {t("services.shellInstanceAny")}
            </SelectItem>
            {instances.map((instance) => (
              <SelectItem key={instance.id} value={instance.id}>
                {instanceLabel(instance.id)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {/* Re-key on the effective target so switching instances tears down the old
          terminal and opens a fresh one against the new pod. */}
      <WebShellTerminal
        key={effective}
        serviceId={serviceId}
        instanceId={instanceId}
      />
    </div>
  );
}

// instanceLabel renders Render's compact "Instance <suffix>" form from the
// opaque srv-…-<suffix> id.
function instanceLabel(id: string): string {
  const suffix = id.split("-").pop() ?? id;
  return `Instance ${suffix}`;
}
