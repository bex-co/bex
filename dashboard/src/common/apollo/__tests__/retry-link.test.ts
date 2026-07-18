import { describe, it, expect } from "vitest";
import { ApolloLink, Observable, gql, type ApolloClient } from "@apollo/client";
import { ServerError } from "@apollo/client/errors";
import { createRetryLink } from "../retry-link";

const QUERY = gql`
  query Thing {
    thing {
      id
    }
  }
`;
const MUTATION = gql`
  mutation Move {
    move {
      ok
    }
  }
`;

/**
 * A terminal link that fails `failures` times with `error`, then succeeds.
 * Counts attempts so the tests can assert exactly how often the retry link
 * re-forwarded the operation.
 */
function flakyLink(error: unknown, failures: number) {
  let attempts = 0;
  const link = new ApolloLink(
    () =>
      new Observable((observer) => {
        attempts += 1;
        if (attempts <= failures) {
          observer.error(error);
          return;
        }
        observer.next({ data: { ok: true } });
        observer.complete();
      }),
  );
  return { link, attempts: () => attempts };
}

/** Run `request` through retry→terminal and settle to result or error. */
function run(
  terminal: ApolloLink,
  request: ApolloLink.Request,
): Promise<{ result?: unknown; error?: unknown }> {
  const chain = ApolloLink.from([createRetryLink(), terminal]);
  return new Promise((resolve) => {
    ApolloLink.execute(chain, request, {
      client: {} as ApolloClient,
    }).subscribe({
      next: (result) => resolve({ result }),
      error: (error) => resolve({ error }),
    });
  });
}

function serverError(status: number): ServerError {
  return new ServerError(`status ${status}`, {
    response: new Response("nope", { status }),
    bodyText: "nope",
  });
}

describe("createRetryLink (w1/m52 t003)", () => {
  it("retries a query past transport failures", async () => {
    const flaky = flakyLink(new TypeError("fetch failed"), 2);
    const { result, error } = await run(flaky.link, { query: QUERY });
    expect(error).toBeUndefined();
    expect(result).toEqual({ data: { ok: true } });
    expect(flaky.attempts()).toBe(3);
  });

  it("gives up after the retry budget and surfaces the error", async () => {
    const flaky = flakyLink(new TypeError("fetch failed"), 99);
    const { error } = await run(flaky.link, { query: QUERY });
    expect(error).toBeInstanceOf(TypeError);
    expect(flaky.attempts()).toBe(3);
  });

  it("retries a query past a 5xx (the LB's roll-window 502)", async () => {
    const flaky = flakyLink(serverError(502), 1);
    const { result } = await run(flaky.link, { query: QUERY });
    expect(result).toEqual({ data: { ok: true } });
    expect(flaky.attempts()).toBe(2);
  });

  it("does NOT retry 4xx — a 401 is an answer, not a blip", async () => {
    const flaky = flakyLink(serverError(401), 99);
    const { error } = await run(flaky.link, { query: QUERY });
    expect(ServerError.is(error)).toBe(true);
    expect(flaky.attempts()).toBe(1);
  });

  it("never retries a mutation, even on a transport failure", async () => {
    const flaky = flakyLink(new TypeError("fetch failed"), 99);
    const { error } = await run(flaky.link, { query: MUTATION });
    expect(error).toBeInstanceOf(TypeError);
    expect(flaky.attempts()).toBe(1);
  });

  it("passes GraphQL execution errors through without retrying", async () => {
    let attempts = 0;
    const graphQLErrorLink = new ApolloLink(
      () =>
        new Observable((observer) => {
          attempts += 1;
          observer.next({ errors: [{ message: "forbidden" }] });
          observer.complete();
        }),
    );
    const { result, error } = await run(graphQLErrorLink, { query: QUERY });
    expect(error).toBeUndefined();
    expect(result).toEqual({ errors: [{ message: "forbidden" }] });
    expect(attempts).toBe(1);
  });
});
