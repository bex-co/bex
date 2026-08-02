import { ApolloLink } from "@apollo/client";
import { Observable } from "rxjs";
import { dataBoundary, type DataBoundary } from "./data-boundary";

export function createBoundaryLink(boundary: DataBoundary = dataBoundary) {
  return new ApolloLink((operation, forward) => {
    const lease = boundary.begin();
    const context = operation.getContext();
    operation.setContext({
      fetchOptions: { ...context.fetchOptions, signal: lease.signal },
    });

    return new Observable((observer) => {
      const subscription = forward(operation).subscribe({
        next: (value) => {
          if (lease.isCurrent()) observer.next(value);
        },
        error: (error) => {
          if (lease.isCurrent()) observer.error(error);
          else observer.complete();
          lease.finish();
        },
        complete: () => {
          observer.complete();
          lease.finish();
        },
      });
      return () => {
        subscription.unsubscribe();
        lease.finish();
      };
    });
  });
}
