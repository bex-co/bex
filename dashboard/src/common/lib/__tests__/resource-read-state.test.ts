import { describe, it, expect, beforeEach } from "vitest";
import { ServerError } from "@apollo/client/errors";
import { resourceNotFound, resourceReadState } from "../resource-read-state";
import {
  resetOwnDeletesForTests,
  trackOwnDelete,
} from "@/common/lib/own-delete";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";

const notFoundErr = codedGraphQLError("NOT_FOUND");
const forbidden = codedGraphQLError("FORBIDDEN");
const outage = new Error("Failed to fetch");
const unauthorized = new ServerError("status 401", {
  response: new Response("no", { status: 401 }),
  bodyText: "no",
});
const proxy404 = new ServerError("status 404", {
  response: new Response("no", { status: 404 }),
  bodyText: "no",
});

beforeEach(() => {
  resetOwnDeletesForTests();
});

// w6/m44: bex-api answers a dead id with `null` AND an errors entry saying
// why, so "settled empty" and "no error" are different questions.
describe("resourceNotFound (w6/m44)", () => {
  it("a dead id is not found even though the backend reports it as an error", () => {
    expect(resourceNotFound(null, false, notFoundErr)).toBe(true);
  });

  it("an empty settle with no error at all is still not found", () => {
    expect(resourceNotFound(null, false, undefined)).toBe(true);
  });

  // The code decides (w5/m130): a feature's own *_NOT_FOUND counts, and the
  // wording alone, without a code, does not.
  it("decides not found by code, not by 'not found' in the wording", () => {
    expect(
      resourceNotFound(
        null,
        false,
        codedGraphQLError("AGENT_SESSION_NOT_FOUND"),
      ),
    ).toBe(true);
    expect(
      resourceNotFound(null, false, uncodedGraphQLError("deploy not found")),
    ).toBe(false);
  });

  it("is never true while loading, once resolved, or on a failure", () => {
    expect(resourceNotFound(null, true, notFoundErr)).toBe(false);
    expect(resourceNotFound({ id: "srv-1" }, false, notFoundErr)).toBe(false);
    expect(resourceNotFound(null, false, outage)).toBe(false);
    expect(resourceNotFound(null, false, forbidden)).toBe(false);
  });
});

// w3/m706: which of Render's states a resource page draws for its read.
describe("resourceReadState (w3/m706)", () => {
  it("draws nothing while loading or once it has the resource", () => {
    expect(resourceReadState(null, true, notFoundErr)).toBeNull();
    expect(resourceReadState({ id: "srv-1" }, false, outage)).toBeNull();
  });

  it("a dead id, coded or errorless, is not found", () => {
    expect(resourceReadState(null, false, notFoundErr)).toBe("not-found");
    expect(resourceReadState(null, false, undefined)).toBe("not-found");
  });

  it("a refused read is forbidden: Render's Access denied", () => {
    expect(resourceReadState(null, false, forbidden)).toBe("forbidden");
  });

  // w3/m80: an expired session never wears the outage's clothes.
  it("an expired session is unauthenticated, not an error", () => {
    expect(resourceReadState(null, false, unauthorized)).toBe(
      "unauthenticated",
    );
  });

  // bex-api answers a missing resource 200 with a coded error, so a transport
  // 404 is an ingress or proxy failure, and neither is the wording alone.
  it("any other failure is an error", () => {
    expect(resourceReadState(null, false, outage)).toBe("error");
    expect(resourceReadState(null, false, proxy404)).toBe("error");
    expect(
      resourceReadState(null, false, uncodedGraphQLError("deploy not found")),
    ).toBe("error");
  });

  // Apollo caches a refused read's null without its error: the loader that
  // saw the refusal decides over an errorless empty read.
  it("the route's refusal stands over an errorless empty read", () => {
    expect(resourceReadState(null, false, undefined, "forbidden")).toBe(
      "forbidden",
    );
    expect(resourceReadState(null, false, undefined, "not-found")).toBe(
      "not-found",
    );
    // A live read's own answer still wins, and a resource that came back
    // (membership granted since) is no state at all.
    expect(resourceReadState(null, false, notFoundErr, "forbidden")).toBe(
      "not-found",
    );
    expect(
      resourceReadState({ id: "srv-1" }, false, undefined, "forbidden"),
    ).toBeNull();
  });

  // w4/645: the dashboard's own delete owns the toast and the landing.
  it("stands down on not found while an own delete is in flight", async () => {
    let finish: () => void = () => {};
    const deleting = trackOwnDelete(
      () => new Promise<void>((resolve) => (finish = resolve)),
    );
    expect(resourceReadState(null, false, notFoundErr)).toBeNull();
    // Only not found: a failure during the delete is still drawn.
    expect(resourceReadState(null, false, outage)).toBe("error");
    finish();
    await deleting;
  });

  it("stays down in the grace after an own delete succeeded", async () => {
    await trackOwnDelete(async () => undefined);
    expect(resourceReadState(null, false, notFoundErr)).toBeNull();
  });

  it("re-arms at once when an own delete fails", async () => {
    await expect(
      trackOwnDelete(async () => {
        throw new Error("refused");
      }),
    ).rejects.toThrow("refused");
    expect(resourceReadState(null, false, notFoundErr)).toBe("not-found");
  });
});
