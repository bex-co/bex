// Apollo 4 still sends refetch() of a skipped (standby) query to the network,
// as network-only. A read gated behind `skip` must gate its refetch the same
// way, or a recovery pass would request exactly what the gate withholds.
export function refetchUnlessSkipped<T>(
  skip: boolean,
  refetch: () => Promise<T>,
): Promise<T | undefined> {
  return skip ? Promise.resolve(undefined) : refetch();
}
