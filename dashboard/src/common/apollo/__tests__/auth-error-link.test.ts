import { describe, it, expect, vi } from "vitest";
import {
  ApolloLink,
  Observable,
  gql,
  type ApolloClient,
} from "@apollo/client";
import { ServerError } from "@apollo/client/errors";
import {
  createAuthErrorLink,
  isUnauthenticatedError,
} from "../auth-error-link";

const QUERY = gql`
  query Thing {
    thing {
      id
    }
  }
`;

function serverError(status: number): ServerError {
  return new ServerError(`status ${status}`, {
    response: new Response("nope", { status }),
    bodyText: "nope",
  });
}

/** A terminal link that fails with `error`. */
function terminalError(error: unknown): ApolloLink {
  return new ApolloLink(
    () =>
      new Observable((observer) => {
        observer.error(error);
      }),
  );
}

/** A terminal link that succeeds. */
function terminalOk(): ApolloLink {
  return new ApolloLink(
    () =>
      new Observable((observer) => {
        observer.next({ data: { ok: true } });
        observer.complete();
      }),
  );
}

function run(
  onUnauthorized: () => void,
  terminal: ApolloLink,
): Promise<{ result?: unknown; error?: unknown }> {
  const chain = ApolloLink.from([createAuthErrorLink(onUnauthorized), terminal]);
  return new Promise((resolve) => {
    ApolloLink.execute(
      chain,
      { query: QUERY },
      { client: {} as ApolloClient },
    ).subscribe({
      next: (result) => resolve({ result }),
      error: (error) => resolve({ error }),
    });
  });
}

describe("isUnauthenticatedError (w3/m80 t002)", () => {
  it("is true only for a 401 ServerError", () => {
    expect(isUnauthenticatedError(serverError(401))).toBe(true);
    expect(isUnauthenticatedError(serverError(403))).toBe(false);
    expect(isUnauthenticatedError(serverError(500))).toBe(false);
  });

  it("unwraps a 401 nested behind a cause chain", () => {
    const wrapped = Object.assign(new Error("wrapper"), {
      cause: serverError(401),
    });
    expect(isUnauthenticatedError(wrapped)).toBe(true);
  });

  it("is false for a plain error, a GraphQL-shaped error, and non-errors", () => {
    expect(isUnauthenticatedError(new Error("Failed to fetch"))).toBe(false);
    expect(isUnauthenticatedError({ message: "forbidden" })).toBe(false);
    expect(isUnauthenticatedError(undefined)).toBe(false);
    expect(isUnauthenticatedError(null)).toBe(false);
  });
});

describe("createAuthErrorLink (w3/m80 t001)", () => {
  it("signals a 401 and still surfaces the error", async () => {
    const onUnauthorized = vi.fn();
    const { error } = await run(onUnauthorized, terminalError(serverError(401)));

    expect(onUnauthorized).toHaveBeenCalledTimes(1);
    expect(ServerError.is(error)).toBe(true);
  });

  it("does not signal on a 5xx — that is a blip, not an expiry", async () => {
    const onUnauthorized = vi.fn();
    const { error } = await run(onUnauthorized, terminalError(serverError(502)));

    expect(onUnauthorized).not.toHaveBeenCalled();
    expect(ServerError.is(error)).toBe(true);
  });

  it("passes a successful result through untouched", async () => {
    const onUnauthorized = vi.fn();
    const { result, error } = await run(onUnauthorized, terminalOk());

    expect(error).toBeUndefined();
    expect(result).toEqual({ data: { ok: true } });
    expect(onUnauthorized).not.toHaveBeenCalled();
  });
});
