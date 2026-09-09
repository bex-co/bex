import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
  gql,
} from "@apollo/client";
import { createAuthLinks, type AuthCredentials } from "../auth-links";

const PING = gql`
  query Ping {
    __typename
  }
`;

function buildClient(
  credentials: AuthCredentials,
  fetchImpl: typeof fetch,
): ApolloClient {
  const { authLink, refreshLink } = createAuthLinks(credentials);
  return new ApolloClient({
    cache: new InMemoryCache(),
    link: ApolloLink.from([
      refreshLink,
      authLink,
      new HttpLink({
        uri: "https://example.test/graphql",
        fetch: fetchImpl,
      }),
    ]),
  });
}

async function capture(run: () => Promise<unknown>): Promise<unknown> {
  try {
    await run();
    return undefined;
  } catch (error) {
    return error;
  }
}

describe("createAuthLinks 401 recovery", () => {
  it("retries once with a fresh bearer after a successful forceRefresh", async () => {
    let token = "expired";
    let signedOut = 0;
    const authorizations: Array<string | null> = [];
    let calls = 0;

    const fetchImpl: typeof fetch = async (_input, init) => {
      calls += 1;
      const headers = new Headers(init?.headers);
      const authorization = headers.get("authorization");
      authorizations.push(authorization);
      if (calls === 1) {
        return new Response(
          JSON.stringify({ errors: [{ message: "expired" }] }),
          {
            status: 401,
            headers: { "content-type": "application/json" },
          },
        );
      }
      return new Response(JSON.stringify({ data: { __typename: "Query" } }), {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    };

    const client = buildClient(
      {
        getAccessToken: async () => token,
        forceRefresh: async () => {
          token = "fresh";
        },
        signOut: async () => {
          signedOut += 1;
        },
      },
      fetchImpl,
    );

    const result = await client.query({
      query: PING,
      fetchPolicy: "network-only",
    });
    expect(result.data).toEqual({ __typename: "Query" });
    expect(authorizations).toEqual(["Bearer expired", "Bearer fresh"]);
    expect(signedOut).toBe(0);
  });

  it("signs out only when forceRefresh fails", async () => {
    let signedOut = 0;
    let calls = 0;
    const fetchImpl: typeof fetch = async () => {
      calls += 1;
      return new Response(
        JSON.stringify({ errors: [{ message: "expired" }] }),
        {
          status: 401,
          headers: { "content-type": "application/json" },
        },
      );
    };

    const client = buildClient(
      {
        getAccessToken: async () => "expired",
        forceRefresh: async () => {
          throw new Error("refresh failed");
        },
        signOut: async () => {
          signedOut += 1;
        },
      },
      fetchImpl,
    );

    const error = await capture(() =>
      client.query({ query: PING, fetchPolicy: "network-only" }),
    );
    expect(error).toBeTruthy();
    expect(calls).toBe(1);
    expect(signedOut).toBe(1);
  });

  it("does not sign out when the retried request fails for a non-auth reason", async () => {
    let token = "expired";
    let signedOut = 0;
    let calls = 0;
    const fetchImpl: typeof fetch = async () => {
      calls += 1;
      if (calls === 1) {
        return new Response(
          JSON.stringify({ errors: [{ message: "expired" }] }),
          {
            status: 401,
            headers: { "content-type": "application/json" },
          },
        );
      }
      return new Response(JSON.stringify({ errors: [{ message: "boom" }] }), {
        status: 500,
        headers: { "content-type": "application/json" },
      });
    };

    const client = buildClient(
      {
        getAccessToken: async () => token,
        forceRefresh: async () => {
          token = "fresh";
        },
        signOut: async () => {
          signedOut += 1;
        },
      },
      fetchImpl,
    );

    const error = await capture(() =>
      client.query({ query: PING, fetchPolicy: "network-only" }),
    );
    expect(error).toBeTruthy();
    expect(calls).toBe(2);
    expect(signedOut).toBe(0);
    expect(token).toBe("fresh");
  });

  it("preserves an explicit logout bearer and skips refresh", async () => {
    let refreshed = 0;
    let signedOut = 0;
    let calls = 0;
    const authorizations: Array<string | null> = [];
    const fetchImpl: typeof fetch = async (_input, init) => {
      calls += 1;
      const headers = new Headers(init?.headers);
      authorizations.push(headers.get("authorization"));
      return new Response(
        JSON.stringify({ errors: [{ message: "expired" }] }),
        {
          status: 401,
          headers: { "content-type": "application/json" },
        },
      );
    };

    const client = buildClient(
      {
        getAccessToken: async () => "should-not-use",
        forceRefresh: async () => {
          refreshed += 1;
        },
        signOut: async () => {
          signedOut += 1;
        },
      },
      fetchImpl,
    );

    const error = await capture(() =>
      client.query({
        query: PING,
        fetchPolicy: "network-only",
        context: {
          headers: { authorization: "Bearer logout-token" },
          skipAuthRefresh: true,
        },
      }),
    );
    expect(error).toBeTruthy();
    expect(authorizations).toEqual(["Bearer logout-token"]);
    expect(calls).toBe(1);
    expect(refreshed).toBe(0);
    expect(signedOut).toBe(0);
  });

  it("does not retry a second 401 after authRetried", async () => {
    let token = "expired";
    let refreshed = 0;
    let signedOut = 0;
    let calls = 0;
    const fetchImpl: typeof fetch = async () => {
      calls += 1;
      return new Response(
        JSON.stringify({ errors: [{ message: "expired" }] }),
        {
          status: 401,
          headers: { "content-type": "application/json" },
        },
      );
    };

    const client = buildClient(
      {
        getAccessToken: async () => token,
        forceRefresh: async () => {
          refreshed += 1;
          token = "fresh";
        },
        signOut: async () => {
          signedOut += 1;
        },
      },
      fetchImpl,
    );

    const error = await capture(() =>
      client.query({ query: PING, fetchPolicy: "network-only" }),
    );
    expect(error).toBeTruthy();
    expect(calls).toBe(2);
    expect(refreshed).toBe(1);
    expect(signedOut).toBe(0);
  });
});
