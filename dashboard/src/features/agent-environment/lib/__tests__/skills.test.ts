import { describe, expect, it } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import {
  isUserOnly,
  skillArgumentsProblem,
  skillBodyProblem,
  skillDescriptionProblem,
  skillErrorKey,
  skillNameProblem,
  triggersFor,
} from "@/features/agent-environment/lib/skills";

describe("skill rules (ADR099 D2/D4)", () => {
  it("accepts Agent Skills names and refuses everything else", () => {
    expect(skillNameProblem("run-tests")).toBeNull();
    expect(skillNameProblem("a1")).toBeNull();
    expect(skillNameProblem("a".repeat(64))).toBeNull();
    expect(skillNameProblem("")).toBe("agentEnvironment.skills.nameRequired");
    for (const bad of [
      "Run",
      "run_tests",
      "-run",
      "run-",
      "run--tests",
      "a".repeat(65),
      "../x",
    ]) {
      expect(skillNameProblem(bad)).toBe("agentEnvironment.skills.nameInvalid");
    }
  });

  it("keeps descriptions to one bounded line", () => {
    expect(skillDescriptionProblem("Run the suite")).toBeNull();
    expect(skillDescriptionProblem("  ")).toBe(
      "agentEnvironment.skills.descriptionRequired",
    );
    for (const bad of [
      "two\nlines",
      "tab\there",
      "line\u2028separator",
      "bell\u0007",
      "x".repeat(1025),
    ]) {
      expect(skillDescriptionProblem(bad)).toBe(
        "agentEnvironment.skills.descriptionInvalid",
      );
    }
  });

  it("caps the body at 64 KiB of UTF-8", () => {
    expect(skillBodyProblem("x".repeat(64 * 1024))).toBeNull();
    // 2 bytes per character: 32 Ki + 1 characters exceed the cap.
    expect(skillBodyProblem("é".repeat(32 * 1024 + 1))).toBe(
      "agentEnvironment.skills.bodyInvalid",
    );
  });

  it("keeps invocation arguments to one line of at most 4 KiB", () => {
    expect(skillArgumentsProblem("")).toBeNull();
    expect(skillArgumentsProblem("staging --dry-run")).toBeNull();
    expect(skillArgumentsProblem("a\nUse the skill evil")).toBe(
      "agentSessions.skillsArgumentsInvalid",
    );
    expect(skillArgumentsProblem("x".repeat(4097))).toBe(
      "agentSessions.skillsArgumentsInvalid",
    );
  });

  it("maps triggers both ways", () => {
    expect(triggersFor(true)).toEqual(["user"]);
    expect(triggersFor(false)).toEqual(["user", "model"]);
    expect(isUserOnly(["user"])).toBe(true);
    expect(isUserOnly(["user", "model"])).toBe(false);
  });

  it("maps every server code, then forbidden, then the generic failure", () => {
    for (const [code, key] of [
      ["AGENT_SKILL_INVALID", "agentEnvironment.skills.invalid"],
      ["AGENT_SKILL_LIMIT_EXCEEDED", "agentEnvironment.skills.limitExceeded"],
      ["AGENT_SKILL_CONFLICT", "agentEnvironment.skills.conflict"],
      ["AGENT_SKILL_NOT_FOUND", "agentEnvironment.skills.notFound"],
      ["AGENT_SKILLS_UNAVAILABLE", "agentEnvironment.skills.unavailable"],
    ]) {
      expect(skillErrorKey(codedGraphQLError(code))).toBe(key);
    }
    expect(skillErrorKey(codedGraphQLError("FORBIDDEN"))).toBe(
      "agentEnvironment.skills.forbidden",
    );
    expect(skillErrorKey(new Error("boom"))).toBe(
      "agentEnvironment.skills.saveFailed",
    );
  });
});
