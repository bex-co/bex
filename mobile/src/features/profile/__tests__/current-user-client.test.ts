import { CurrentUserClient, CurrentUserError } from "../current-user-client";

// Fixtures deliberately carry no real personal data, tokens, or OAuth subjects.
const okUser = { name: "Test Person", email: "person@example.test" };

function client(
  responses: Array<Response | (() => never)>,
  hooks: { onRefresh?: () => void; tokens?: string[] } = {},
) {
  const tokens = hooks.tokens ?? ["t0"];
  const requests: string[] = [];
  const impl = new CurrentUserClient(
    "https://api.bex.co",
    {
      getAccessToken: async () =>
        tokens[Math.min(requests.length, tokens.length - 1)],
      forceRefresh: async () => {
        hooks.onRefresh?.();
      },
    },
    (async (_input: RequestInfo, init?: RequestInit) => {
      requests.push((init?.headers as Record<string, string>)?.Authorization);
      const next = responses.shift();
      if (!next) throw new Error("unexpected request");
      if (typeof next === "function") return next();
      return next;
    }) as typeof fetch,
  );
  return { impl, requests };
}

async function capture(run: () => Promise<unknown>): Promise<unknown> {
  try {
    await run();
    return undefined;
  } catch (error) {
    return error;
  }
}

describe("CurrentUserClient", () => {
  it("returns the validated identity for a signed-in caller", async () => {
    const { impl } = client([
      new Response(JSON.stringify(okUser), { status: 200 }),
    ]);
    const user = await impl.fetch(new AbortController().signal);
    expect(user).toEqual(okUser);
  });

  it("refreshes once after a 401 then retries with the new token", async () => {
    let refreshes = 0;
    const { impl, requests } = client(
      [
        new Response("{}", { status: 401 }),
        new Response(JSON.stringify(okUser), { status: 200 }),
      ],
      { onRefresh: () => (refreshes += 1), tokens: ["old", "new"] },
    );
    const user = await impl.fetch(new AbortController().signal);
    expect(user).toEqual(okUser);
    expect(refreshes).toBe(1);
    expect(requests).toEqual(["Bearer old", "Bearer new"]);
  });

  it("classifies a transport failure as offline (network), not unavailable", async () => {
    const { impl } = client([
      () => {
        throw new Error("connection reset");
      },
    ]);
    const error = await capture(() => impl.fetch(new AbortController().signal));
    expect(error instanceof CurrentUserError).toBe(true);
    expect((error as CurrentUserError).code).toBe("network");
  });

  it("reports a 503 as unavailable and a persistent 403 as auth", async () => {
    const unavailable = client([new Response("{}", { status: 503 })]);
    expect(
      (
        (await capture(() =>
          unavailable.impl.fetch(new AbortController().signal),
        )) as CurrentUserError
      ).code,
    ).toBe("unavailable");

    const denied = client(
      [
        new Response("{}", { status: 401 }),
        new Response("{}", { status: 403 }),
      ],
      { tokens: ["a", "b"] },
    );
    expect(
      (
        (await capture(() =>
          denied.impl.fetch(new AbortController().signal),
        )) as CurrentUserError
      ).code,
    ).toBe("auth");
  });

  it("treats a malformed body as unavailable rather than crashing", async () => {
    const notJson = client([new Response("<html>", { status: 200 })]);
    expect(
      (
        (await capture(() =>
          notJson.impl.fetch(new AbortController().signal),
        )) as CurrentUserError
      ).code,
    ).toBe("unavailable");

    const wrongShape = client([
      new Response(JSON.stringify([1, 2]), { status: 200 }),
    ]);
    expect(
      (
        (await capture(() =>
          wrongShape.impl.fetch(new AbortController().signal),
        )) as CurrentUserError
      ).code,
    ).toBe("unavailable");
  });
});
