import { useCallback, useEffect, useRef, useState } from "react";
import {
  SafeActionExecutor,
  type SafeActionOperation,
  type SafeActionOutcome,
} from "./executor";
import {
  confirmSafeAction,
  createSafeActionIntent,
  type RetryIdentityFactory,
  type SafeActionIntent,
  type SafeActionTarget,
} from "./model";
import type { SafeActionDefinition } from "./registry";
import { useActionAccess } from "./use-action-access";

export interface UseSafeActionOptions<Data> {
  operation: SafeActionOperation<Data>;
  identityFactory?: RetryIdentityFactory;
  executor?: SafeActionExecutor;
}

export interface UseSafeActionResult<Data> {
  intent: SafeActionIntent | null;
  outcome: SafeActionOutcome<Data> | null;
  pending: boolean;
  requestConfirmation: (
    definition: SafeActionDefinition,
    target: SafeActionTarget,
  ) => void;
  dismissConfirmation: () => void;
  confirm: () => Promise<SafeActionOutcome<Data> | null>;
  retry: () => Promise<SafeActionOutcome<Data> | null>;
  dismissOutcome: () => void;
  cancelInFlight: () => void;
}

/** Owns one confirmation/result lifecycle while the executor dedupes the screen. */
export function useSafeAction<Data>(
  options: UseSafeActionOptions<Data>,
): UseSafeActionResult<Data> {
  const executorRef = useRef(options.executor ?? new SafeActionExecutor());
  const optionsRef = useRef(options);
  optionsRef.current = options;
  const [intent, setIntent] = useState<SafeActionIntent | null>(null);
  const intentRef = useRef<SafeActionIntent | null>(null);
  intentRef.current = intent;
  const [outcome, setOutcome] = useState<SafeActionOutcome<Data> | null>(null);
  const [, renderPending] = useState(0);
  const executor = executorRef.current;
  const access = useActionAccess();
  const accessRef = useRef(access);
  accessRef.current = access;
  const binding = useRef<string | null>(null);

  useEffect(
    () => executor.subscribe(() => renderPending((value) => value + 1)),
    [executor],
  );
  useEffect(() => () => executor.cancelAll(), [executor]);
  useEffect(() => {
    if (
      !intent ||
      (binding.current && access.isCurrent(binding.current, intent.actionId))
    )
      return;
    executor.cancelAll();
    binding.current = null;
    setIntent(null);
    setOutcome(null);
  });

  const requestConfirmation = useCallback(
    (definition: SafeActionDefinition, target: SafeActionTarget) => {
      const current = accessRef.current;
      if (!current.isCurrent(current.key, definition.id)) return;
      binding.current = current.key;
      setOutcome(null);
      setIntent(
        createSafeActionIntent(
          definition,
          target,
          optionsRef.current.identityFactory,
        ),
      );
    },
    [],
  );
  const dismissConfirmation = useCallback(() => {
    if (!executor.isPending(intentRef.current)) setIntent(null);
  }, [executor]);
  const execute = useCallback(
    async (next: SafeActionIntent) => {
      const confirmedBinding = binding.current;
      if (
        !confirmedBinding ||
        !accessRef.current.isCurrent(confirmedBinding, next.actionId)
      )
        return null;
      setIntent(next);
      const result = await executor.execute(next, (context) => {
        if (!accessRef.current.isCurrent(confirmedBinding, next.actionId))
          throw new Error("mobile access is not currently verified");
        return optionsRef.current.operation(context);
      });
      if (!accessRef.current.isCurrent(confirmedBinding, next.actionId))
        return null;
      setOutcome(result);
      return result;
    },
    [executor],
  );
  const confirm = useCallback(() => {
    const current = intentRef.current;
    return current
      ? execute(current.confirmed ? current : confirmSafeAction(current))
      : Promise.resolve(null);
  }, [execute]);
  const retry = useCallback(() => {
    const current = intentRef.current;
    return current?.confirmed ? execute(current) : Promise.resolve(null);
  }, [execute]);
  const dismissOutcome = useCallback(() => setOutcome(null), []);
  const cancelInFlight = useCallback(() => {
    const current = intentRef.current;
    if (current) executor.cancel(current);
  }, [executor]);

  return {
    intent,
    outcome,
    pending: executor.isPending(intent),
    requestConfirmation,
    dismissConfirmation,
    confirm,
    retry,
    dismissOutcome,
    cancelInFlight,
  };
}
