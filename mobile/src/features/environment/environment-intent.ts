import {
  confirmSafeAction,
  createSafeActionIntent,
  type SafeActionIntent,
} from "../../components/safe-action/model";
import { defineSafeAction } from "../../components/safe-action/registry";

export const updateEnvironmentVariableAction = defineSafeAction(
  "update-environment-variable",
  "service",
);

export type RevealedEnvironmentVariable = Readonly<{
  serviceId: string;
  key: string;
  value: string;
  revision: string;
}>;

export type EnvironmentEditIntent = Readonly<{
  action: SafeActionIntent;
  serviceId: string;
  key: string;
  value: string;
  revision: string;
}>;

type Listener = (value: RevealedEnvironmentVariable | null) => void;

/**
 * Holds at most one revealed value in process memory. There is deliberately no
 * serializer: replacing the key or crossing a data/lifecycle boundary calls
 * clear(), after which the value is unreachable from this session.
 */
export class EnvironmentSecretSession {
  private current: RevealedEnvironmentVariable | null = null;
  private listeners = new Set<Listener>();

  value(): RevealedEnvironmentVariable | null {
    return this.current;
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  reveal(value: RevealedEnvironmentVariable): void {
    validateRevealed(value);
    this.current = Object.freeze({ ...value });
    this.emit();
  }

  edit(value: string): void {
    if (!this.current) return;
    this.current = Object.freeze({ ...this.current, value });
    this.emit();
  }

  clear(): void {
    if (!this.current) return;
    this.current = null;
    this.emit();
  }

  private emit(): void {
    for (const listener of this.listeners) listener(this.current);
  }
}

export function createEnvironmentEditIntent(
  revealed: RevealedEnvironmentVariable,
  serviceLabel: string,
): EnvironmentEditIntent {
  validateRevealed(revealed);
  const label = `${serviceLabel.trim()} · ${revealed.key}`;
  const action = createSafeActionIntent(updateEnvironmentVariableAction, {
    kind: "service",
    id: revealed.serviceId,
    label,
  });
  return Object.freeze({ action, ...revealed });
}

export function confirmEnvironmentEditIntent(
  intent: EnvironmentEditIntent,
): EnvironmentEditIntent {
  return Object.freeze({ ...intent, action: confirmSafeAction(intent.action) });
}

function validateRevealed(value: RevealedEnvironmentVariable): void {
  if (!value.serviceId.trim() || !value.key.trim() || !value.revision.trim()) {
    throw new Error("service, key, and revision are required");
  }
}
