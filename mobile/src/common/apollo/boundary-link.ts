import { ApolloLink } from "@apollo/client";
import { Observable } from "rxjs";
import { dataBoundary, type DataBoundary } from "./data-boundary";

export function createBoundaryLink(boundary: DataBoundary = dataBoundary) {
  return new ApolloLink((operation, forward) => {
    const lease = boundary.begin();
    const context = operation.getContext();
    const callerSignal = (
      context.fetchOptions as { signal?: AbortSignal } | undefined
    )?.signal;
    const combinedSignal = combineAbortSignals(callerSignal, lease.signal);
    operation.setContext({
      fetchOptions: {
        ...context.fetchOptions,
        signal: combinedSignal.signal,
      },
    });

    const finish = () => {
      combinedSignal.cleanup();
      lease.finish();
    };

    return new Observable((observer) => {
      const subscription = forward(operation).subscribe({
        next: (value) => {
          if (lease.isCurrent()) observer.next(value);
        },
        error: (error) => {
          if (lease.isCurrent()) observer.error(error);
          else observer.complete();
          finish();
        },
        complete: () => {
          observer.complete();
          finish();
        },
      });
      return () => {
        subscription.unsubscribe();
        finish();
      };
    });
  });
}

export function combineAbortSignals(
  caller: AbortSignal | undefined,
  boundary: AbortSignal,
): { signal: AbortSignal; cleanup: () => void } {
  if (!caller) return { signal: boundary, cleanup: () => undefined };
  const controller = new AbortController();
  const cleanup = () => {
    caller.removeEventListener("abort", abort);
    boundary.removeEventListener("abort", abort);
  };
  const abort = () => {
    cleanup();
    controller.abort();
  };
  if (caller.aborted || boundary.aborted) {
    abort();
  } else {
    caller.addEventListener("abort", abort, { once: true });
    boundary.addEventListener("abort", abort, { once: true });
  }
  return { signal: controller.signal, cleanup };
}
