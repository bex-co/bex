import { describe, it, expect, vi } from "vitest";
import { ApolloLink, Observable, gql, type ApolloClient } from "@apollo/client";
import { ServerError } from "@apollo/client/errors";
import {
  createAuthErrorLink,
  isEmailVerificationRequiredError,
  isUnauthenticatedError,
  workspaceTwoFactorRequiredIds,
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
  const chain = ApolloLink.from([
    createAuthErrorLink(onUnauthorized),
    terminal,
  ]);
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
    const { error } = await run(
      onUnauthorized,
      terminalError(serverError(401)),
    );

    expect(onUnauthorized).toHaveBeenCalledTimes(1);
    expect(ServerError.is(error)).toBe(true);
  });

  it("does not signal on a 5xx — that is a blip, not an expiry", async () => {
    const onUnauthorized = vi.fn();
    const { error } = await run(
      onUnauthorized,
      terminalError(serverError(502)),
    );

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

// ADR075 D8 revision (w2/m168): bex-api's auth middleware refuses an
// unverified human with a REST-shaped 403 body BEFORE GraphQL runs, so on
// /graphql it arrives as a transport ServerError, never a GraphQL error.
const VERIFICATION_BODY = JSON.stringify({
  error: "forbidden",
  message: "verify your email address to continue",
  id: "email-verification-required",
  code: "EMAIL_VERIFICATION_REQUIRED",
  params: {},
});

function serverErrorWithBody(status: number, bodyText: string): ServerError {
  return new ServerError(`status ${status}`, {
    response: new Response(bodyText, { status }),
    bodyText,
  });
}

function runWithVerification(terminal: ApolloLink): Promise<{
  result?: unknown;
  error?: unknown;
  onUnauthorized: ReturnType<typeof vi.fn>;
  onVerify: ReturnType<typeof vi.fn>;
}> {
  const onUnauthorized = vi.fn();
  const onVerify = vi.fn();
  const chain = ApolloLink.from([
    createAuthErrorLink(onUnauthorized, onVerify),
    terminal,
  ]);
  return new Promise((resolve) => {
    ApolloLink.execute(
      chain,
      { query: QUERY },
      { client: {} as ApolloClient },
    ).subscribe({
      next: (result) => resolve({ result, onUnauthorized, onVerify }),
      error: (error) => resolve({ error, onUnauthorized, onVerify }),
    });
  });
}

describe("isEmailVerificationRequiredError (w2/m168)", () => {
  it("is true only for a 403 whose JSON body carries the code", () => {
    expect(
      isEmailVerificationRequiredError(
        serverErrorWithBody(403, VERIFICATION_BODY),
      ),
    ).toBe(true);
    // Same body on another status, or another 403 code, is not the wall.
    expect(
      isEmailVerificationRequiredError(
        serverErrorWithBody(401, VERIFICATION_BODY),
      ),
    ).toBe(false);
    expect(
      isEmailVerificationRequiredError(
        serverErrorWithBody(
          403,
          JSON.stringify({ code: "ACCOUNT_DELETION_PENDING" }),
        ),
      ),
    ).toBe(false);
  });

  it("is false for a non-JSON 403 body and for a message-only match", () => {
    expect(
      isEmailVerificationRequiredError(
        serverErrorWithBody(403, "EMAIL_VERIFICATION_REQUIRED"),
      ),
    ).toBe(false);
    expect(
      isEmailVerificationRequiredError(serverErrorWithBody(403, "null")),
    ).toBe(false);
    expect(
      isEmailVerificationRequiredError(
        new Error("EMAIL_VERIFICATION_REQUIRED"),
      ),
    ).toBe(false);
    expect(isEmailVerificationRequiredError(undefined)).toBe(false);
  });

  it("unwraps a cause chain", () => {
    const wrapped = Object.assign(new Error("wrapper"), {
      cause: serverErrorWithBody(403, VERIFICATION_BODY),
    });
    expect(isEmailVerificationRequiredError(wrapped)).toBe(true);
  });
});

describe("createAuthErrorLink — verification backstop (w2/m168)", () => {
  it("routes a transport 403 EMAIL_VERIFICATION_REQUIRED to the wall and still surfaces it", async () => {
    const { error, onUnauthorized, onVerify } = await runWithVerification(
      terminalError(serverErrorWithBody(403, VERIFICATION_BODY)),
    );
    expect(onVerify).toHaveBeenCalledTimes(1);
    expect(onUnauthorized).not.toHaveBeenCalled();
    expect(ServerError.is(error)).toBe(true);
  });

  it("routes a GraphQL extensions.code refusal to the wall and passes the result on", async () => {
    const refused = {
      data: null,
      errors: [
        {
          message: "verify your email address to continue",
          extensions: { code: "EMAIL_VERIFICATION_REQUIRED" },
        },
      ],
    };
    const { result, onVerify } = await runWithVerification(
      new ApolloLink(
        () =>
          new Observable((observer) => {
            observer.next(refused);
            observer.complete();
          }),
      ),
    );
    expect(onVerify).toHaveBeenCalledTimes(1);
    expect(result).toEqual(refused);
  });

  it("does not route an ordinary 403 or a 401 to the wall", async () => {
    const forbidden = await runWithVerification(
      terminalError(
        serverErrorWithBody(403, JSON.stringify({ code: "FORBIDDEN" })),
      ),
    );
    expect(forbidden.onVerify).not.toHaveBeenCalled();

    const expired = await runWithVerification(
      terminalError(serverErrorWithBody(401, "nope")),
    );
    expect(expired.onVerify).not.toHaveBeenCalled();
    expect(expired.onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it("does not signal on a successful result", async () => {
    const { onVerify } = await runWithVerification(terminalOk());
    expect(onVerify).not.toHaveBeenCalled();
  });
});

// w22/m402: bex-api refuses a member with no second factor in a workspace that
// requires one from inside a resolver, so on /graphql it is a GraphQL error
// whose extensions carry {code, workspaceId}: never a transport status.
describe("createAuthErrorLink — workspace two-factor (w22/m402)", () => {
  function refused(
    errors: Array<{ message: string; extensions?: Record<string, unknown> }>,
  ): ApolloLink {
    return new ApolloLink(
      () =>
        new Observable((observer) => {
          observer.next({ data: null, errors });
          observer.complete();
        }),
    );
  }

  async function runTwoFactor(terminal: ApolloLink) {
    const onUnauthorized = vi.fn();
    const onVerify = vi.fn();
    const onTwoFactor = vi.fn();
    const chain = ApolloLink.from([
      createAuthErrorLink(onUnauthorized, onVerify, onTwoFactor),
      terminal,
    ]);
    const outcome = await new Promise<{ result?: unknown; error?: unknown }>(
      (resolve) => {
        ApolloLink.execute(
          chain,
          { query: QUERY },
          { client: {} as ApolloClient },
        ).subscribe({
          next: (result) => resolve({ result }),
          error: (error) => resolve({ error }),
        });
      },
    );
    return { ...outcome, onUnauthorized, onVerify, onTwoFactor };
  }

  it("reports the refused workspace and passes the result on untouched", async () => {
    const errors = [
      {
        message: "this workspace requires two-factor authentication",
        extensions: { code: "WORKSPACE_2FA_REQUIRED", workspaceId: "tea-acme" },
      },
    ];
    const { result, onTwoFactor, onUnauthorized, onVerify } =
      await runTwoFactor(refused(errors));

    expect(onTwoFactor).toHaveBeenCalledTimes(1);
    expect(onTwoFactor).toHaveBeenCalledWith("tea-acme");
    expect(result).toEqual({ data: null, errors });
    expect(onUnauthorized).not.toHaveBeenCalled();
    expect(onVerify).not.toHaveBeenCalled();
  });

  it("reports each refused workspace of one result", async () => {
    const { onTwoFactor } = await runTwoFactor(
      refused([
        {
          message: "a",
          extensions: { code: "WORKSPACE_2FA_REQUIRED", workspaceId: "tea-a" },
        },
        {
          message: "b",
          extensions: { code: "WORKSPACE_2FA_REQUIRED", workspaceId: "tea-b" },
        },
      ]),
    );
    expect(onTwoFactor.mock.calls).toEqual([["tea-a"], ["tea-b"]]);
  });

  it("ignores every other refusal, including other 403s", async () => {
    // Another GraphQL code, even one that names a workspace.
    const forbidden = await runTwoFactor(
      refused([
        {
          message: "forbidden",
          extensions: { code: "FORBIDDEN", workspaceId: "tea-acme" },
        },
      ]),
    );
    expect(forbidden.onTwoFactor).not.toHaveBeenCalled();

    // A transport 403, even one whose body carries a code.
    const transport = await runTwoFactor(
      terminalError(
        serverErrorWithBody(
          403,
          JSON.stringify({ code: "WORKSPACE_2FA_REQUIRED" }),
        ),
      ),
    );
    expect(transport.onTwoFactor).not.toHaveBeenCalled();
    expect(transport.onUnauthorized).not.toHaveBeenCalled();

    // The unverified-email wall keeps its own handler.
    const verification = await runTwoFactor(
      refused([
        {
          message: "verify",
          extensions: { code: "EMAIL_VERIFICATION_REQUIRED" },
        },
      ]),
    );
    expect(verification.onVerify).toHaveBeenCalledTimes(1);
    expect(verification.onTwoFactor).not.toHaveBeenCalled();

    // A message that merely mentions it, and no errors at all.
    const wording = await runTwoFactor(
      refused([{ message: "WORKSPACE_2FA_REQUIRED" }]),
    );
    expect(wording.onTwoFactor).not.toHaveBeenCalled();
    expect(
      (await runTwoFactor(terminalOk())).onTwoFactor,
    ).not.toHaveBeenCalled();
  });

  it("does not wall a workspace it cannot name", () => {
    for (const workspaceId of [undefined, "", 7, null, { id: "tea-a" }]) {
      expect(
        workspaceTwoFactorRequiredIds({
          errors: [
            {
              extensions: { code: "WORKSPACE_2FA_REQUIRED", workspaceId },
            },
          ],
        }),
      ).toEqual([]);
    }
  });
});
