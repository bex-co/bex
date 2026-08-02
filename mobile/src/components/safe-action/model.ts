import type {
  MobileSafeActionId,
  SafeActionDefinition,
  SafeActionTargetKind,
} from "./registry";

export interface SafeActionTarget {
  kind: SafeActionTargetKind;
  id: string;
  /** Already-localized display name. It is never used as backend identity. */
  label: string;
}

export interface SafeActionIntent {
  actionId: MobileSafeActionId;
  target: Readonly<SafeActionTarget>;
  retryIdentity: string;
  confirmationKey: string;
  confirmed: boolean;
}

export type RetryIdentityFactory = (
  actionId: MobileSafeActionId,
  target: SafeActionTarget,
) => string;

let identitySequence = 0;
const defaultRetryIdentity: RetryIdentityFactory = (actionId, target) => {
  identitySequence += 1;
  return `${actionId}:${target.kind}:${target.id}:${Date.now().toString(36)}:${identitySequence.toString(36)}`;
};

function confirmationKey(
  actionId: MobileSafeActionId,
  target: SafeActionTarget,
): string {
  return `${actionId}\u0000${target.kind}\u0000${target.id}`;
}

export function createSafeActionIntent(
  definition: SafeActionDefinition,
  target: SafeActionTarget,
  identityFactory: RetryIdentityFactory = defaultRetryIdentity,
): SafeActionIntent {
  if (target.kind !== definition.targetKind) {
    throw new Error(
      `mobile action ${definition.id} cannot target ${target.kind}`,
    );
  }
  if (!target.id.trim() || !target.label.trim()) {
    throw new Error("safe action target id and label are required");
  }
  const retryIdentity = identityFactory(definition.id, target).trim();
  if (!retryIdentity) throw new Error("safe action retry identity is required");
  const immutableTarget = Object.freeze({ ...target });
  return Object.freeze({
    actionId: definition.id,
    target: immutableTarget,
    retryIdentity,
    confirmationKey: confirmationKey(definition.id, immutableTarget),
    confirmed: false,
  });
}

/** Binds confirmation to the exact action and target shown by the dialog. */
export function confirmSafeAction(intent: SafeActionIntent): SafeActionIntent {
  if (
    intent.confirmationKey !== confirmationKey(intent.actionId, intent.target)
  ) {
    throw new Error("safe action confirmation no longer matches its target");
  }
  return Object.freeze({ ...intent, confirmed: true });
}

export function safeActionOperationKey(intent: SafeActionIntent): string {
  return intent.confirmationKey;
}
