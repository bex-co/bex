// An allowing datastore lifecycle gate (w4/m578) for unit tests that render a
// database or Key Value page without a live Apollo client: every verb may
// run, and dispatch runs it without a recheck.
export function mockAllowedLifecycleGate() {
  return {
    useLifecycleGate: () => ({
      reason: () => undefined,
      dispatch: <T>(_verb: string, run: () => Promise<T>) => run(),
      checking: false,
    }),
  };
}
