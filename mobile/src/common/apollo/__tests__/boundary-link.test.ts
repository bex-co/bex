import { ApolloLink, type OperationVariables } from "@apollo/client";
import { Observable } from "rxjs";
import { createBoundaryLink } from "../boundary-link";
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
