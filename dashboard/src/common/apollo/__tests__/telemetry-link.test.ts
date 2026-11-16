import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ApolloLink,
  Observable,
  gql,
  type ApolloClient,
  type DocumentNode,
} from "@apollo/client";
import { ServerError } from "@apollo/client/errors";
import { CHECKOUT_KEY } from "@/common/lib/telemetry-router";
import { MUTATION_EVENTS, createTelemetryLink } from "../telemetry-link";

const CREATE_SERVICE = gql`
  mutation CreateService($name: String!, $type: String) {
    createService(name: $name, type: $type) {
      id
    }
  }
`;
const SET_ENV_VAR = gql`
  mutation SetEnvVar($id: String!) {
    setEnvVar(id: $id) {
      id
    }
  }
`;
const SERVICES = gql`
  query Services {
    services {
      id
    }
  }
`;

type Terminal = { result: unknown } | { error: unknown };

function run(
  query: DocumentNode,
  variables: Record<string, unknown>,
  terminal: Terminal,
): Promise<{ result?: unknown; error?: unknown }> {
  const chain = ApolloLink.from([
    createTelemetryLink(),
    new ApolloLink(
      () =>
        new Observable((observer) => {
          if ("error" in terminal) observer.error(terminal.error);
          else {
            observer.next(terminal.result as never);
            observer.complete();
          }
        }),
    ),
  ]);
  return new Promise((resolve) => {
    ApolloLink.execute(
      chain,
      { query, variables },
      { client: {} as ApolloClient },
    ).subscribe({
      next: (result) => resolve({ result }),
      error: (error) => resolve({ error }),
    });
  });
}

describe("createTelemetryLink", () => {
  let gtag: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    gtag = vi.fn();
    window.gtag = gtag as unknown as Window["gtag"];
    window.sessionStorage.clear();
  });
  afterEach(() => {
    delete window.gtag;
  });

  it("reports a mapped mutation's success with bounded labels only", async () => {
    const ok = { data: { createService: { id: "srv-abc" } } };
    const { result } = await run(
      CREATE_SERVICE,
      {
        name: "my secret project",
        type: "web_service",
        runtime: "node",
        plan: "starter",
        repo: "https://github.com/acme/app",
      },
      { result: ok },
    );
    expect(result).toEqual(ok);
    expect(gtag.mock.calls).toEqual([
      [
        "event",
        "service_created",
        {
          service_type: "web_service",
          runtime: "node",
          plan: "starter",
          source: "git",
        },
      ],
    ]);
  });

  it("reports nothing for an unmapped mutation that succeeded, or a query", async () => {
    await run(SET_ENV_VAR, { id: "srv-abc" }, { result: { data: {} } });
    await run(SERVICES, {}, { result: { errors: [{ message: "boom" }] } });
    await run(SERVICES, {}, { error: new Error("Failed to fetch") });
    expect(gtag).not.toHaveBeenCalled();
  });

  it("reports a refused mutation by operation and error code, mapped or not", async () => {
    const refused = {
      errors: [{ message: "nope", extensions: { code: "PAYMENT_REQUIRED" } }],
    };
    const { result } = await run(
      CREATE_SERVICE,
      { name: "x" },
      { result: refused },
    );
    await run(
      SET_ENV_VAR,
      { id: "srv-abc" },
      { result: { errors: [{ message: "free text, not a code" }] } },
    );
    expect(result).toEqual(refused);
    expect(gtag.mock.calls).toEqual([
      [
        "event",
        "action_failed",
        { action: "CreateService", error_code: "PAYMENT_REQUIRED" },
      ],
      [
        "event",
        "action_failed",
        { action: "SetEnvVar", error_code: "UNKNOWN" },
      ],
    ]);
  });

  it("reports a transport failure and still surfaces it; a cancel is not one", async () => {
    const down = new ServerError("status 502", {
      response: new Response("nope", { status: 502 }),
      bodyText: "nope",
    });
    const { error } = await run(CREATE_SERVICE, { name: "x" }, { error: down });
    await run(SET_ENV_VAR, {}, { error: new TypeError("Failed to fetch") });
    await run(
      SET_ENV_VAR,
      {},
      { error: new DOMException("aborted", "AbortError") },
    );
    expect(error).toBe(down);
    expect(gtag.mock.calls.map(([, , params]) => params)).toEqual([
      { action: "CreateService", error_code: "HTTP_502" },
      { action: "SetEnvVar", error_code: "NETWORK" },
    ]);
  });

  it("remembers that a checkout began, for the return to count", async () => {
    const BEGIN = gql`
      mutation CreateBillingCheckoutSession($workspaceId: String!) {
        createBillingCheckoutSession(workspaceId: $workspaceId) {
          url
        }
      }
    `;
    await run(BEGIN, { workspaceId: "tea-1" }, { result: { data: {} } });
    expect(gtag).toHaveBeenCalledWith("event", "begin_checkout", {});
    expect(window.sessionStorage.getItem(CHECKOUT_KEY)).toBe("1");
  });
});

// A renamed or removed mutation would silently stop being counted.
describe("MUTATION_EVENTS", () => {
  it("names only mutations the dashboard defines", () => {
    const src = join(import.meta.dirname, "../../..");
    const defined = new Set<string>();
    for (const path of readdirSync(src, {
      recursive: true,
      encoding: "utf8",
    })) {
      if (!path.endsWith(".graphql")) continue;
      const text = readFileSync(join(src, path), "utf8");
      for (const [, name] of text.matchAll(/^\s*mutation\s+(\w+)/gm)) {
        defined.add(name);
      }
    }
    expect(Object.keys(MUTATION_EVENTS).filter((n) => !defined.has(n))).toEqual(
      [],
    );
  });
});
