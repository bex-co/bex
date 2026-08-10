/**
 * Test-only replacement for `@tanstack/start-fn-stubs`, aliased in
 * vitest.config.ts.
 *
 * The real package is the runtime fallback used when the TanStack Start
 * compiler plugin has not rewritten `createIsomorphicFn` call sites — and it
 * is server-biased: once `.server(...)` appears anywhere in the chain, the
 * stub always invokes the server implementation. Vitest runs the un-compiled
 * sources in jsdom, so that bias makes every isomorphic helper call its
 * server branch (`getRequest()` etc.) and throw outside a request context.
 *
 * Tests are the client environment, so this mirror of the upstream stub
 * prefers the client implementation instead.
 */

type RuntimeFn<TArgs extends Array<unknown>, TReturn> = ((
  ...args: TArgs
) => TReturn) & {
  client: (impl: (...args: TArgs) => TReturn) => RuntimeFn<TArgs, TReturn>;
  server: (impl: (...args: TArgs) => TReturn) => RuntimeFn<TArgs, TReturn>;
};

function createRuntimeFn<TArgs extends Array<unknown>, TReturn>(
  fn: (...args: TArgs) => TReturn,
  clientImpl: ((...args: TArgs) => TReturn) | undefined,
): RuntimeFn<TArgs, TReturn> {
  return Object.assign(fn, {
    client: (nextClientImpl: (...args: TArgs) => TReturn) =>
      createRuntimeFn(nextClientImpl, nextClientImpl),
    server: (serverImpl: (...args: TArgs) => TReturn) =>
      createRuntimeFn(clientImpl ?? serverImpl, clientImpl),
  });
}

export function createIsomorphicFn(): RuntimeFn<Array<unknown>, unknown> {
  return createRuntimeFn<Array<unknown>, unknown>(() => undefined, undefined);
}

export const createClientOnlyFn = <T>(fn: T): T => fn;
export const createServerOnlyFn = <T>(fn: T): T => fn;
