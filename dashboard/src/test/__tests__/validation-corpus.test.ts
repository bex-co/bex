import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { isValidCIDR } from "@/common/lib/cidr";
import { isValidDockerImageRef } from "@/common/lib/utils/docker-image";
import { isValidGitUrl } from "@/common/lib/utils/git-url";
import { isValidLabel } from "@/common/lib/utils/resource-label";
import { envValueRuleError } from "@/features/services/lib/environment-draft";
import {
  relativePathError,
  rootDirError,
} from "@/features/services/lib/build-settings-validation";
import { domainNameError } from "@/features/services/lib/custom-domain-name";
import { registryHostError } from "@/features/registry-credentials/lib/credential-fields";
import {
  CORPUS_DIR,
  corpusFamilies,
  corpusMismatches,
  readCorpus,
  type CorpusCase,
} from "../validation-corpus";

// The client validator of each family in lego/backend/testdata/validation, as a
// form calls it. The Go side asserts the same files against the server
// (lego/backend/internal/*/validation_corpus_test.go), so a form and bex-api
// cannot disagree on an input: a new form adds a case to its file, a new family
// adds a file and a row here.
type Client = (text: string, c: CorpusCase) => boolean;

const CLIENTS: Record<string, Client> = {
  label: isValidLabel,
  cidr: isValidCIDR,
  "root-dir": (text) => rootDirError(text) === null,
  "build-path": (text) => relativePathError(text) === null,
  "env-value": (text, c) => envValueRuleError(c.key ?? "", text) === null,
  // A blank host is the dialog's own required check, not a host error.
  "registry-host": (text) =>
    text.trim() !== "" && registryHostError(text) === null,
  "repo-url": isValidGitUrl,
  "docker-image": isValidDockerImageRef,
  "custom-domain": (text) => domainNameError(text) === null,
};

const FAMILIES = Object.keys(CLIENTS).sort();

describe("the validation corpus", () => {
  it("has a client validator for every family, and a family for every client", () => {
    expect(corpusFamilies()).toEqual(FAMILIES);
  });

  // A corpus-only change must run the dashboard suite, which reads it, as it
  // runs the backend suite (its tree is `lego/backend/**`, which
  // scripts/ci_test_inputs.py holds): the dashboard workflow lists the corpus
  // once for push and once for pull_request. The path is composed here, so that
  // script cannot check it.
  it("is a test input of the dashboard suite", () => {
    const workflow = readFileSync(
      `${process.cwd()}/../.github/workflows/dashboard-test.yml`,
      "utf8",
    );
    const entry = `- "${CORPUS_DIR}/*.json"`;
    expect(workflow.split(entry).length - 1).toBe(2);
  });

  describe.each(FAMILIES)("%s", (family) => {
    const cases = readCorpus(family);

    it("agrees with the client validator on every case", () => {
      expect(corpusMismatches(cases, CLIENTS[family])).toEqual([]);
    });

    it("holds enough cases of each kind for the self-test to mean something", () => {
      expect(cases.length).toBeGreaterThanOrEqual(20);
      expect(cases.some((c) => c.valid)).toBe(true);
      expect(cases.some((c) => !c.valid && !c.serverOnly)).toBe(true);
      expect(cases.every((c) => !(c.valid && c.serverOnly))).toBe(true);
    });

    // A client that refuses one valid input, or accepts one input the server
    // refuses where the client has no say, must fail the corpus.
    it("fails a client one case stricter than the server", () => {
      const valid = cases.find((c) => c.valid)!;
      const stricter: Client = (text, c) =>
        CLIENTS[family](text, c) && c !== valid;
      expect(corpusMismatches(cases, stricter)).not.toEqual([]);
    });

    it("fails a client one case looser than the server", () => {
      const refused = cases.find((c) => !c.valid && !c.serverOnly)!;
      const looser: Client = (text, c) =>
        CLIENTS[family](text, c) || c === refused;
      expect(corpusMismatches(cases, looser)).not.toEqual([]);
    });
  });
});

describe("corpusMismatches", () => {
  const cases: CorpusCase[] = [
    { input: "ok", valid: true },
    { input: "refused", valid: false },
    { input: "server-only", valid: false, serverOnly: true },
  ];
  const verdict =
    (accepted: string[]): Client =>
    (text) =>
      accepted.includes(text);

  it("passes a client that matches the server", () => {
    expect(corpusMismatches(cases, verdict(["ok"]))).toEqual([]);
  });

  it("lets the client accept what only the server can judge", () => {
    expect(corpusMismatches(cases, verdict(["ok", "server-only"]))).toEqual([]);
  });

  it("names a client that refuses what the server accepts", () => {
    expect(corpusMismatches(cases, verdict([]))).toEqual([
      expect.stringContaining(
        '"ok": the client refuses it, the server accepts',
      ),
    ]);
  });

  it("names a client that accepts what the server refuses", () => {
    expect(corpusMismatches(cases, verdict(["ok", "refused"]))).toEqual([
      expect.stringContaining(
        '"refused": the client accepts it, the server refuses',
      ),
    ]);
  });
});
