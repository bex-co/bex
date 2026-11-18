import { useCallback, useState } from "react";
import { toast } from "sonner";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import {
  graphQLErrorCodes,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import type { LoginRequirementOutcomeCopy } from "@/features/workspaces/lib/login-requirements";

export interface LoginRequirementRefusalText {
  /** Already in the user's language: locale copy for the coded refusals, else
   *  the server's own answer or a generic line (never a raw transport error). */
  text: string;
  /** The lock-out guard: the acting admin fixes it on their own account. */
  accountFix: boolean;
}

export interface UseLoginRequirementSetterResult {
  /** Turns the requirement on or off. */
  setRequired: (enabled: boolean) => Promise<void>;
  /** The mutation is in flight. */
  busy: boolean;
  /** The refusal, shown inline beside the switch it came from. */
  error: LoginRequirementRefusalText | null;
}

/** Words a failure `copy.refusals` does not name: `fallback` unless the
 *  surface knows better. */
type RefusalMessage = (err: unknown, t: Translate, fallback: string) => string;

const serverMessage: RefusalMessage = (err, _t, fallback) =>
  mutationErrorMessage(err, fallback);

/**
 * What a login-requirement switch does around its mutation, the same for a
 * workspace's (w22/m402, w7/m465) and an org's (w7/m466): a toast naming
 * `name` once it is written, or the mutation's coded refusal
 * (`copy.refusals`) in the card's own words, kept until the next attempt. Any
 * other failure is worded by `refusalMessage` (the server's answer, by
 * default). `run` sends the mutation, and `busy` is its in-flight flag; keep
 * `run` stable (`useCallback`).
 */
export function useLoginRequirementSetter({
  run,
  busy,
  copy,
  name,
  refusalMessage = serverMessage,
}: {
  run: (enabled: boolean) => Promise<unknown>;
  busy: boolean;
  copy: LoginRequirementOutcomeCopy;
  name: string;
  refusalMessage?: RefusalMessage;
}): UseLoginRequirementSetterResult {
  const { t } = useTranslations();
  const [error, setError] = useState<LoginRequirementRefusalText | null>(null);

  const setRequired = useCallback(
    async (enabled: boolean) => {
      setError(null);
      try {
        await run(enabled);
        toast.success(
          t(enabled ? copy.enabledToastKey : copy.disabledToastKey, { name }),
        );
      } catch (err) {
        const code = graphQLErrorCodes(err).find((c) =>
          Object.hasOwn(copy.refusals, c),
        );
        const coded = code ? copy.refusals[code] : undefined;
        setError(
          coded
            ? { text: t(coded.messageKey), accountFix: coded.accountFix }
            : {
                text: refusalMessage(err, t, t(copy.errorKey)),
                accountFix: false,
              },
        );
      }
    },
    [run, copy, name, refusalMessage, t],
  );

  return { setRequired, busy, error };
}
