import { ApolloLink, type OperationVariables } from "@apollo/client";
import { Observable } from "rxjs";
import { combineAbortSignals, createBoundaryLink } from "../boundary-link";
import { DataBoundary } from "../data-boundary";

describe("boundary Apollo link", () => {
  it("never emits a response that arrives after workspace invalidation", async () => {
    const boundary = new DataBoundary();
    const link = createBoundaryLink(boundary);
    let context: Record<string, unknown> = {};
    let emit: ((value: ApolloLink.Result) => void) | undefined;
    const operation = {
      getContext: () => context,
      setContext: (next: Record<string, unknown>) => {
        context = { ...context, ...next };
      },
      variables: {} as OperationVariables,
    } as unknown as ApolloLink.Operation;
    const values: ApolloLink.Result[] = [];
    link
      .request(
        operation,
        () =>
          new Observable((observer) => {
            emit = (value) => observer.next(value);
          }),
      )
      ?.subscribe((value) => values.push(value));

    emit?.({ data: { workspace: "tea-old" } });
    await boundary.reset("tea-new");
    emit?.({ data: { workspace: "tea-old-late" } });

    expect(values).toEqual([{ data: { workspace: "tea-old" } }]);
    const fetchOptions = context.fetchOptions as { signal: AbortSignal };
    expect(fetchOptions.signal.aborted).toBe(true);
  });
});

describe("combineAbortSignals", () => {
  it("preserves both the caller timeout and identity boundary", () => {
    const caller = new AbortController();
    const boundary = new AbortController();
    const callerCombined = combineAbortSignals(caller.signal, boundary.signal);
    caller.abort();
    expect(callerCombined.signal.aborted).toBe(true);

    const nextCaller = new AbortController();
    const nextBoundary = new AbortController();
    const boundaryCombined = combineAbortSignals(
      nextCaller.signal,
      nextBoundary.signal,
    );
    nextBoundary.abort();
    expect(boundaryCombined.signal.aborted).toBe(true);
  });

  it("detaches both upstream listeners when an operation completes", () => {
    const caller = new AbortController();
    const boundary = new AbortController();
    const combined = combineAbortSignals(caller.signal, boundary.signal);

    combined.cleanup();
    caller.abort();
    boundary.abort();

    expect(combined.signal.aborted).toBe(false);
  });
});
