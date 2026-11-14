import { describe, expect, it } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import {
  MAX_REPO_ENVIRONMENTS,
  REPO_STEP_LIMITS,
  STEP_NAME_PATTERN,
  repoEnvironmentErrorKey,
  repoEnvironmentProblem,
  repoStepProblems,
  trimRepoSteps,
} from "@/features/agent-environment/lib/repo-environments";
import { goIntConstant, goRegexp } from "@/test/go-source";

const P = "agentEnvironment.repoEnvs.";

describe("repoStepProblems (ADR109 D3)", () => {
  it.each([
    ["deps", null],
    ["Build 2", null],
    ["lint.ci", null],
    ["type_check-all", null],
    ["9lives", null],
    ["a".repeat(64), null],
    ["a".repeat(65), `${P}nameInvalid`],
    ["-deps", `${P}nameInvalid`],
    [".hidden", `${P}nameInvalid`],
    ["deps/ci", `${P}nameInvalid`],
    ["déps", `${P}nameInvalid`],
  ])("judges the name %j", (name, problem) => {
    expect(repoStepProblems("setup", [{ name, run: "true" }])[0].name).toBe(
      problem,
    );
  });

  it("judges trimmed values, as the card sends them", () => {
    expect(
      repoStepProblems("setup", [{ name: "  deps ", run: " npm ci\n" }]),
    ).toEqual([{ name: null, run: null }]);
  });

  it("flags a repeated name on its later rows only", () => {
    expect(
      repoStepProblems("checks", [
        { name: "test", run: "npm test" },
        { name: "lint", run: "npm run lint" },
        { name: "test", run: "npm t" },
      ]).map((p) => p.name),
    ).toEqual([null, null, `${P}nameDuplicate`]);
  });

  // The two lists are separate: setup "test" and check "test" may coexist.
  it("scopes uniqueness to one list", () => {
    expect(repoStepProblems("setup", [{ name: "test", run: "x" }])).toEqual([
      { name: null, run: null },
    ]);
  });

  it("requires both halves of a row that has either", () => {
    expect(
      repoStepProblems("setup", [
        { name: "deps", run: "  " },
        { name: "", run: "npm ci" },
      ]),
    ).toEqual([
      { name: null, run: `${P}runRequired` },
      { name: `${P}nameRequired`, run: null },
    ]);
  });

  it("ignores a blank row, including for duplicates", () => {
    expect(
      repoStepProblems("setup", [
        { name: " ", run: "" },
        { name: "", run: "" },
      ]),
    ).toEqual([
      { name: null, run: null },
      { name: null, run: null },
    ]);
  });

  // Each list's own byte cap, counted in UTF-8 bytes as the server's len().
  it.each([
    ["setup", 8 * 1024, `${P}setupRunTooLong`],
    ["checks", 2 * 1024, `${P}checkRunTooLong`],
  ] as const)("caps a %s command at %i bytes", (kind, max, problem) => {
    const run = (text: string) =>
      repoStepProblems(kind, [{ name: "a", run: text }])[0].run;
    expect(run("x".repeat(max))).toBeNull();
    expect(run("x".repeat(max + 1))).toBe(problem);
    // "é" is two bytes: half as many fit.
    expect(run("é".repeat(max / 2))).toBeNull();
    expect(run("é".repeat(max / 2 + 1))).toBe(problem);
    expect(run("echo a\0b")).toBe(problem);
  });
});

describe("repoEnvironmentProblem", () => {
  const step = (name: string) => ({ name, run: "true" });
  const many = (n: number) =>
    Array.from({ length: n }, (_, i) => step(`s${i}`));

  it("needs at least one step or check", () => {
    expect(repoEnvironmentProblem([], [])).toBe(`${P}environmentEmpty`);
    expect(repoEnvironmentProblem([step("deps")], [])).toBeNull();
    expect(repoEnvironmentProblem([], [step("test")])).toBeNull();
  });

  it("caps each list at 10", () => {
    expect(repoEnvironmentProblem(many(10), many(10))).toBeNull();
    expect(repoEnvironmentProblem(many(11), [])).toBe(`${P}setupTooMany`);
    expect(repoEnvironmentProblem([], many(11))).toBe(`${P}checksTooMany`);
  });
});

describe("trimRepoSteps", () => {
  it("keeps the order, trims, and drops blank rows", () => {
    expect(
      trimRepoSteps([
        { name: " build ", run: " npm run build\n" },
        { name: "", run: "   " },
        { name: "deps", run: "npm ci" },
      ]),
    ).toEqual([
      { name: "build", run: "npm run build" },
      { name: "deps", run: "npm ci" },
    ]);
  });
});

describe("repoEnvironmentErrorKey", () => {
  it.each([
    [{ field: "repo" }, `${P}repoInvalid`],
    [{ field: "setup", index: 2 }, `${P}setupRefused`],
    [{ field: "checks", name: "test" }, `${P}checksRefused`],
    [{ field: "other" }, `${P}invalid`],
    [{}, `${P}invalid`],
  ])("words AGENT_REPO_ENVIRONMENT_INVALID on %j", (params, key) => {
    expect(
      repoEnvironmentErrorKey(
        codedGraphQLError("AGENT_REPO_ENVIRONMENT_INVALID", params),
      ),
    ).toBe(key);
  });

  it.each([
    ["AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED", `${P}limitExceeded`],
    ["AGENT_REPO_ENVIRONMENT_NOT_FOUND", `${P}notFound`],
    ["AGENT_REPO_ENVIRONMENTS_UNAVAILABLE", `${P}unavailable`],
    ["FORBIDDEN", `${P}forbidden`],
    ["INTERNAL", `${P}saveFailed`],
  ])("words %s", (code, key) => {
    expect(repoEnvironmentErrorKey(codedGraphQLError(code))).toBe(key);
  });

  it("falls back for a transport failure", () => {
    expect(repoEnvironmentErrorKey(new Error("Failed to fetch"))).toBe(
      `${P}saveFailed`,
    );
  });
});

// The client mirror of ADR109 D3's limits reads the server's own constants, so
// the two cannot drift apart.
describe("repository environment limits match bex-api", () => {
  const source = "lego/backend/internal/agentsessions/repo_environment.go";
  it("has the same counts, byte limits and name rule", () => {
    expect(MAX_REPO_ENVIRONMENTS).toBe(
      goIntConstant(source, "maxRepoEnvironments"),
    );
    expect(REPO_STEP_LIMITS.setup).toEqual({
      maxSteps: goIntConstant(source, "maxRepoSetupSteps"),
      maxRunBytes: goIntConstant(source, "maxRepoSetupRunBytes"),
    });
    expect(REPO_STEP_LIMITS.checks).toEqual({
      maxSteps: goIntConstant(source, "maxRepoChecks"),
      maxRunBytes: goIntConstant(source, "maxRepoCheckRunBytes"),
    });
    expect(goRegexp(source, "repoStepNamePattern")).toBe(
      STEP_NAME_PATTERN.source,
    );
  });
});
