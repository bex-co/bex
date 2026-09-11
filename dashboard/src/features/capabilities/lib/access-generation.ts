/**
 * Identity / workspace / access generation for the dashboard (w6/m144).
 * Bumped when the active access context changes so late responses and pending
 * confirmations from an obsolete generation cannot restore prior access.
 * Client memory only — not a server membership version.
 */

type Listener = () => void;

let generation = 0;
const listeners = new Set<Listener>();

export function getAccessGeneration(): number {
  return generation;
}

export function bumpAccessGeneration(): number {
  generation += 1;
  for (const listener of listeners) listener();
  return generation;
}

export function subscribeAccessGeneration(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Test-only: reset between suites. */
export function resetAccessGenerationForTests(): void {
  generation = 0;
}
