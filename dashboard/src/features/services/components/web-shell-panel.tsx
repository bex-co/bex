import { type ReactNode, useId, useState } from "react";
import { Lock } from "lucide-react";
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
import type { ShellGate } from "@/features/services/hooks/use-shell-gate";

// ANY_INSTANCE is the sentinel for "let the gateway pick a random Ready replica"
// (the empty-instance case native SSH also uses). Radix Select can't hold an
// empty-string value, so the sentinel maps to `undefined` on the way out.
const ANY_INSTANCE = "any";

/**
 * The Web Shell panel: Render's "Select an instance" picker above the in-browser
 * terminal (w2/m55). Choosing an instance targets that specific replica;
 * "Any ready instance" lets the gateway pick one, matching native SSH. Changing
 * the selection re-opens the terminal against the new target.
 *
 * `initialInstance` is the instance a log line's SSH into instance named
 * (w3/m528): the panel starts on it when it is one of the running instances,
 * and on Any ready instance otherwise. The terminal waits for the first
 * instance list, so it does not open on another pod first.
 *
 * `shell` is the caller's shell decision (w4/726). The terminal mints its
 * ticket only once that allows it: a caller it refuses, such as a Developer
 * in a protected environment, reads the reason in the terminal's place and
 * no connection is attempted. Once open, a routine re-check (checking, or
 * briefly unavailable) leaves the session alone; only a refusal closes it.
 */
export function WebShellPanel({
  serviceId,
  initialInstance,
  shell,
}: {
  serviceId: string;
  initialInstance?: string;
  shell: ShellGate;
}) {
  const { t } = useTranslations();
  const { instances, loading } = useServiceInstances(serviceId);
  const [selected, setSelected] = useState<string>(
    initialInstance ?? ANY_INSTANCE,
  );
  const labelId = useId();
  // Which service's terminal the decision has let open: keyed, so another
  // service's page waits for its own decision before it mints.
  const [openedFor, setOpenedFor] = useState<string | null>(null);
  if (shell.gate.kind === "ready" && openedFor !== serviceId) {
    setOpenedFor(serviceId);
  }
  const opened = openedFor === serviceId;
  const refused =
    shell.gate.kind === "denied" ||
    (!opened && shell.gate.kind !== "ready" && shell.gate.kind !== "checking");
  const awaitingDecision = !opened && shell.gate.kind === "checking";
  const awaitingInstances =
    initialInstance !== undefined && loading && instances.length === 0;

  // A selection that no longer exists (an instance was replaced by a deploy)
  // falls back to "any" so the terminal never targets a vanished pod.
  const known =
    selected === ANY_INSTANCE || instances.some((i) => i.id === selected);
  const effective = known ? selected : ANY_INSTANCE;
  const instanceId = effective === ANY_INSTANCE ? undefined : effective;

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span id={labelId} className="text-muted-foreground text-sm">
          {t("services.shellInstanceLabel")}
        </span>
        <Select
          value={effective}
          onValueChange={setSelected}
          disabled={refused || instances.length === 0}
        >
          <SelectTrigger
            size="sm"
            className="w-[220px]"
            aria-labelledby={labelId}
          >
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
      {refused ? (
        <TerminalFrame testId="web-shell-refused">
          <div className="flex h-96 w-full items-center justify-center gap-2 px-4 text-center text-sm text-neutral-400">
            <Lock className="size-4 shrink-0" aria-hidden />
            <span>{shell.reason}</span>
          </div>
        </TerminalFrame>
      ) : awaitingDecision || awaitingInstances ? (
        // Empty until the shell decision allows it and the instance list
        // says which pod to open on.
        <TerminalFrame
          testId={
            awaitingInstances
              ? "web-shell-awaiting-instances"
              : "web-shell-awaiting-decision"
          }
        />
      ) : (
        <WebShellTerminal
          key={effective}
          serviceId={serviceId}
          instanceId={instanceId}
        />
      )}
    </div>
  );
}

/** The terminal's own frame, with `children` in place of the terminal. */
function TerminalFrame({
  testId,
  children = <div className="h-96 w-full" />,
}: {
  testId: string;
  children?: ReactNode;
}) {
  return (
    <div className="space-y-2" data-testid={testId}>
      <div className="h-5" />
      <div className="overflow-hidden rounded-md border bg-[#0a0a0a] p-2">
        {children}
      </div>
    </div>
  );
}

// instanceLabel renders Render's compact "Instance <suffix>" form from the
// opaque srv-…-<suffix> id.
function instanceLabel(id: string): string {
  const suffix = id.split("-").pop() ?? id;
  return `Instance ${suffix}`;
}
