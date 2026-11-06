/*
 * Copyright 2026.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Approval mode (ADR100 D3–D5). The engine asks the driver's ACP client for
// permission before a tool call; in an autonomous session the driver grants
// everything, as before. In a risky or all session a call the policy gates
// becomes a decision: the driver opens it on the session's Pod-bound decision
// URL, long-polls for a member's answer, and maps it back to the engine's own
// permission option. The wait does not count against the turn timeout (the
// decision clock below).
//
// The risky policy is a guardrail against an agent's mistakes, not a sandbox
// boundary: the sandbox (ADR047 D5) is what confines a determined agent. It
// normalizes the common ways one command is spelled (paths, quotes, wrappers,
// `bash -c`, git's global options) and leaves deliberate obfuscation to that
// boundary.

import os from "node:os";
import path from "node:path";
import { realpathSync } from "node:fs";
import { mkdir, readdir, readFile, rm } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import { setTimeout as sleep } from "node:timers/promises";
import type {
  RequestPermissionRequest,
  RequestPermissionResponse,
} from "@agentclientprotocol/sdk/unstable";
import { describeError } from "./errors.js";
import { writeJsonAtomic } from "./fs-util.js";
import { postToGateway, type GatewayReply } from "./gateway.js";

export type ApprovalMode = "autonomous" | "risky" | "all";

export function approvalMode(raw: string | undefined): ApprovalMode {
  return raw === "risky" || raw === "all" ? raw : "autonomous";
}

// approvalPolicyVersion names the risky-action table below; bump it when a
// rule changes meaning, so a decision's rule id can be read against it.
export const approvalPolicyVersion = 2;

// maxMatchedCommandChars bounds the text the policy parses; anything longer
// asks rather than being scanned (and stands out on its own).
const maxMatchedCommandChars = 16 << 10;

const shells = new Set(["sh", "bash", "zsh", "dash", "ksh", "ash", "fish"]);
const privileged = new Set(["sudo", "doas", "su", "pkexec", "run0"]);
// wrappers run the command that follows them; the policy looks through them.
const wrappers = new Set([
  "env",
  "command",
  "exec",
  "nohup",
  "time",
  "nice",
  "builtin",
]);

// words splits one shell segment into words, honoring quotes and backslashes
// and dropping them, so `git push '--force'` reads as `git push --force`.
function words(text: string): string[] {
  const out: string[] = [];
  let word = "";
  let quote = "";
  let has = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quote) {
      if (c === quote) quote = "";
      else if (c === "\\" && quote === '"' && i + 1 < text.length)
        word += text[++i];
      else word += c;
      continue;
    }
    if (c === "'" || c === '"') {
      quote = c;
      has = true;
    } else if (c === "\\" && i + 1 < text.length) {
      word += text[++i];
      has = true;
    } else if (/\s/.test(c)) {
      if (has || word) out.push(word);
      word = "";
      has = false;
    } else {
      word += c;
      has = true;
    }
  }
  if (has || word) out.push(word);
  return out;
}

// splitTopLevel cuts a command at the separators that start another command
// (; & | newline, &&, ||) and at substitutions ($( <( ` and their close), so a
// command inside one is read on its own. Separators inside quotes stay; a
// substitution inside double quotes still runs, so its body is read unquoted
// and the quote resumes after it.
function splitTopLevel(command: string): string[] {
  const out: string[] = [];
  let current = "";
  let quote = "";
  // resume holds the quote each open substitution interrupted.
  const resume: string[] = [];
  const cut = () => {
    out.push(current);
    current = "";
  };
  for (let i = 0; i < command.length; i++) {
    const c = command[i];
    const opensSubstitution =
      (c === "$" && command[i + 1] === "(") || c === "`";
    if (quote === "'") {
      if (c === "'") quote = "";
      current += c;
      continue;
    }
    if (quote === '"') {
      if (c === '"') {
        quote = "";
        current += c;
      } else if (c === "\\" && i + 1 < command.length) {
        current += c + command[++i];
      } else if (opensSubstitution) {
        resume.push('"');
        quote = "";
        cut();
        if (c === "$") i++;
      } else {
        current += c;
      }
      continue;
    }
    if (c === "'" || c === '"') {
      quote = c;
      current += c;
      continue;
    }
    if (c === "\\" && i + 1 < command.length) {
      current += c + command[++i];
      continue;
    }
    if ((c === ")" || c === "`") && resume.length > 0) {
      cut();
      quote = resume.pop() ?? "";
      continue;
    }
    if (opensSubstitution || (c === "<" && command[i + 1] === "(")) {
      resume.push("");
      cut();
      if (c !== "`") i++;
      continue;
    }
    if (";&|\n)".includes(c)) {
      cut();
      continue;
    }
    current += c;
  }
  cut();
  return out
    .map((segment) => segment.replace(/^"+|"+$/g, "").trim())
    .filter(Boolean);
}

// A Segment is one simple command with its program named by basename
// (/usr/bin/sudo, \sudo and sudo all read as sudo).
interface Segment {
  argv0: string;
  args: string[];
}

function basename(word: string): string {
  return path.basename(word.replace(/^\\+/, ""));
}

// segmentsOf normalizes a command into simple commands: env assignments and
// wrappers are dropped, a privilege wrapper becomes its own segment, and a
// shell's -c script is read as commands too.
function segmentsOf(command: string, depth = 0): Segment[] {
  const out: Segment[] = [];
  for (const piece of splitTopLevel(command)) {
    let argv = words(piece);
    for (;;) {
      while (argv.length > 0 && /^[A-Za-z_][A-Za-z0-9_]*=/.test(argv[0]))
        argv = argv.slice(1);
      if (argv.length === 0) break;
      const name = basename(argv[0]);
      if (privileged.has(name)) {
        out.push({ argv0: name, args: [] });
        argv = argv.slice(1);
        while (argv.length > 0 && argv[0].startsWith("-")) argv = argv.slice(1);
        continue;
      }
      if (wrappers.has(name)) {
        argv = argv.slice(1);
        while (argv.length > 0 && argv[0].startsWith("-")) argv = argv.slice(1);
        continue;
      }
      break;
    }
    if (argv.length === 0) continue;
    const segment = { argv0: basename(argv[0]), args: argv.slice(1) };
    out.push(segment);
    if (shells.has(segment.argv0) && depth < 3) {
      const at = segment.args.findIndex((arg) =>
        /^-[a-zA-Z]*c[a-zA-Z]*$/.test(arg),
      );
      if (at >= 0 && segment.args[at + 1] !== undefined) {
        out.push(...segmentsOf(segment.args[at + 1], depth + 1));
      }
    }
  }
  return out;
}

// pipelinesOf groups a command's stages by pipe, to see what flows into what.
function pipelinesOf(command: string): Segment[][] {
  return command.split(/\|\||&&|[;\n&]/).map((pipeline) =>
    pipeline
      .split("|")
      .map((stage) => segmentsOf(stage).at(-1))
      .filter((s): s is Segment => s !== undefined),
  );
}

// gitSubcommand skips git's global options (-C dir, -c k=v, --git-dir=…).
function gitSubcommand(args: string[]): { sub: string; rest: string[] } {
  let i = 0;
  while (i < args.length && args[i].startsWith("-")) {
    const flag = args[i];
    i += [
      "-C",
      "-c",
      "--git-dir",
      "--work-tree",
      "--namespace",
      "--exec-path",
      "--config-env",
    ].includes(flag)
      ? 2
      : 1;
  }
  return { sub: args[i] ?? "", rest: args.slice(i + 1) };
}

// firstVerb is a CLI's first positional word, skipping flags and the value
// after the given value-taking flags.
function firstVerb(args: string[], valued: string[] = []): string {
  for (let i = 0; i < args.length; i++) {
    if (args[i].startsWith("-")) {
      if (valued.includes(args[i])) i++;
      continue;
    }
    return args[i];
  }
  return "";
}

interface PolicyContext {
  command: string;
  segments: Segment[];
  pipelines: Segment[][];
  cwd: string;
}

interface CommandRule {
  id: string;
  label: string;
  matches: (context: PolicyContext) => string[] | boolean;
}

const gitPush = (s: Segment) =>
  s.argv0 === "git" && gitSubcommand(s.args).sub === "push";

// riskyCommands is the one versioned table of shell commands a risky session
// asks about (ADR100 D3): destructive history rewrites, privilege, publishing,
// infrastructure, cloud and platform API mutations, remote or decoded code
// run in a shell, and deletes outside the checkout. Builds, tests, linters
// and ordinary git work are never listed.
export const riskyCommands: CommandRule[] = [
  {
    id: "git-force-push",
    label: "force-push",
    matches: ({ segments }) =>
      segments.some(
        (s) =>
          gitPush(s) &&
          gitSubcommand(s.args).rest.some(
            (arg) =>
              /^--(force(-with-lease|-if-includes)?|mirror)(=|$)/.test(arg) ||
              /^-[a-zA-Z]*f[a-zA-Z]*$/.test(arg) ||
              /^\+./.test(arg),
          ),
      ),
  },
  {
    id: "git-delete-remote-branch",
    label: "delete a remote branch",
    matches: ({ segments }) =>
      segments.some(
        (s) =>
          gitPush(s) &&
          gitSubcommand(s.args).rest.some(
            (arg) =>
              arg === "--delete" ||
              /^-[a-zA-Z]*d[a-zA-Z]*$/.test(arg) ||
              /^:[^:]/.test(arg),
          ),
      ),
  },
  {
    id: "git-reset-hard-remote",
    label: "reset onto a remote branch",
    matches: ({ segments }) =>
      segments.some((s) => {
        if (s.argv0 !== "git") return false;
        const { sub, rest } = gitSubcommand(s.args);
        return (
          sub === "reset" &&
          rest.includes("--hard") &&
          rest.some((arg) => /^(origin|upstream)\//.test(arg))
        );
      }),
  },
  {
    id: "sudo",
    label: "run as root",
    matches: ({ segments }) => segments.some((s) => privileged.has(s.argv0)),
  },
  {
    id: "package-publish",
    label: "publish a package or release",
    matches: ({ segments }) =>
      segments.some((s) => {
        const verb = firstVerb(s.args);
        switch (s.argv0) {
          case "npm":
          case "yarn":
          case "pnpm":
          case "poetry":
          case "cargo":
            return verb === "publish";
          case "twine":
            return verb === "upload";
          case "gem":
          case "docker":
          case "podman":
            return verb === "push";
          case "gh":
            return (
              verb === "release" &&
              ["create", "upload", "delete", "edit"].includes(
                firstVerb(s.args.slice(s.args.indexOf("release") + 1)),
              )
            );
        }
        return false;
      }),
  },
  {
    id: "infra-mutation",
    label: "change infrastructure",
    matches: ({ segments }) =>
      segments.some((s) => {
        switch (s.argv0) {
          case "kubectl":
          case "oc":
            return [
              "delete",
              "apply",
              "create",
              "replace",
              "patch",
              "scale",
              "drain",
              "cordon",
              "rollout",
              "edit",
              "set",
              "label",
              "annotate",
            ].includes(
              firstVerb(s.args, [
                "-n",
                "--namespace",
                "--context",
                "--kubeconfig",
                "--cluster",
                "--user",
                "-s",
                "--server",
              ]),
            );
          case "terraform":
          case "tofu":
            return ["apply", "destroy", "import"].includes(firstVerb(s.args));
          case "helm":
            return [
              "install",
              "upgrade",
              "uninstall",
              "delete",
              "rollback",
            ].includes(
              firstVerb(s.args, [
                "-n",
                "--namespace",
                "--kube-context",
                "--kubeconfig",
              ]),
            );
        }
        return false;
      }),
  },
  {
    id: "cloud-mutation",
    label: "change a cloud account",
    matches: ({ segments }) =>
      segments.some((s) => {
        const positional = s.args.filter((arg) => !arg.startsWith("-"));
        switch (s.argv0) {
          case "aws":
            return /^(delete|put|create|update|terminate|run-instances|rm|cp|mv|sync|deploy)/.test(
              positional[1] ?? "",
            );
          case "gcloud":
            return positional.some((arg) =>
              ["delete", "create", "deploy", "update"].includes(arg),
            );
          case "az":
            return positional.some((arg) =>
              ["delete", "create", "update"].includes(arg),
            );
          case "render":
          case "bex": {
            const [noun, verb] = positional;
            return (
              (/^deploys?$/.test(noun ?? "") && verb === "create") ||
              (/^services?$/.test(noun ?? "") &&
                ["delete", "update", "suspend"].includes(verb ?? "")) ||
              noun === "restart" ||
              noun === "delete"
            );
          }
        }
        return false;
      }),
  },
  {
    id: "pipe-to-shell",
    label: "run downloaded code",
    matches: ({ pipelines, segments, command }) =>
      pipelines.some((stages) => {
        const from = stages.findIndex(
          (s) => s.argv0 === "curl" || s.argv0 === "wget",
        );
        return (
          from >= 0 && stages.slice(from + 1).some((s) => shells.has(s.argv0))
        );
      }) ||
      // eval "$(curl …)", bash <(curl …), source <(wget …)
      (segments.some((s) => s.argv0 === "curl" || s.argv0 === "wget") &&
        /(\$\(|<\(|`)/.test(command) &&
        segments.some(
          (s) =>
            s.argv0 === "eval" ||
            s.argv0 === "source" ||
            s.argv0 === "." ||
            shells.has(s.argv0),
        )),
  },
  {
    id: "decode-and-run",
    label: "run decoded code",
    matches: ({ pipelines, segments }) => {
      const decodes = (s: Segment) =>
        (s.argv0 === "base64" &&
          s.args.some((arg) => /^(-d|-D|--decode)$/.test(arg))) ||
        (s.argv0 === "xxd" && s.args.some((arg) => /^-r/.test(arg)));
      return (
        pipelines.some((stages) => {
          const from = stages.findIndex(decodes);
          return (
            from >= 0 && stages.slice(from + 1).some((s) => shells.has(s.argv0))
          );
        }) ||
        (segments.some(decodes) && segments.some((s) => s.argv0 === "eval"))
      );
    },
  },
  {
    id: "platform-api-mutation",
    label: "change hosting through its API",
    matches: ({ segments }) =>
      segments.some((s) => {
        if (s.argv0 !== "curl" && s.argv0 !== "wget") return false;
        const host = s.args.some((arg) =>
          /^(https?:\/\/)?(api\.render\.com|api\.bex\.co)([/:?]|$)/i.test(arg),
        );
        if (!host) return false;
        const method = (arg: string, i: number) => {
          if (/^(-X|--request)$/.test(arg)) return s.args[i + 1] ?? "";
          const joined = /^(?:-X|--request=|--method=)(.+)$/.exec(arg);
          return joined ? joined[1] : "";
        };
        return s.args.some(
          (arg, i) =>
            /^(DELETE|POST|PATCH|PUT)$/i.test(method(arg, i)) ||
            /^(-d|--data(-raw|-binary|-urlencode)?|-F|--form|-T|--upload-file|--json|--post-data|--post-file|--body-data|--body-file)(=|$)/.test(
              arg,
            ),
        );
      }),
  },
  {
    id: "rm-outside-checkout",
    label: "delete outside the checkout",
    matches: ({ command, cwd }) => {
      const targets = rmOutside(command, cwd);
      return targets.length > 0 ? targets : false;
    },
  },
];

// The driver's own MCP tools are the session's normal output, so they are
// never gated (ADR100 D3: delivery to the draft PR is the product). A call is
// one only by its exact tool identity, never by a title that merely mentions
// one: Claude names it mcp__bex__<tool> (kind other), Codex sends rawInput
// {server: "bex", tool} with no shell command.
const ownTools = new Set([
  "open_or_update_draft_pr",
  "share_file",
  "submit_result",
  "ask_user",
]);

type ToolCall = RequestPermissionRequest["toolCall"];

export function isOwnTool(toolCall: ToolCall): boolean {
  const kind = toolCall.kind ?? "other";
  if (kind === "edit" || kind === "delete" || kind === "move") return false;
  const raw = toolCall.rawInput as unknown;
  if (raw && typeof raw === "object" && !Array.isArray(raw)) {
    const record = raw as Record<string, unknown>;
    if (
      !("command" in record) &&
      record.server === "bex" &&
      typeof record.tool === "string" &&
      ownTools.has(record.tool)
    ) {
      return true;
    }
  }
  if (kind === "execute") return false;
  const title = (toolCall.title ?? "").trim();
  return (
    title.startsWith("mcp__bex__") &&
    ownTools.has(title.slice("mcp__bex__".length))
  );
}

export interface Gate {
  rule: string;
  title: string;
  detail: Record<string, unknown>;
}

// commandOf reads the shell command an execute call wants to run: the ACP
// adapters put it in rawInput as a string, {command: string} or
// {command: string[]} (an argv like ["bash","-lc","…"]).
export function commandOf(toolCall: ToolCall): string {
  const raw = toolCall.rawInput as unknown;
  if (typeof raw === "string") return raw;
  if (raw && typeof raw === "object") {
    const command = (raw as Record<string, unknown>).command;
    if (typeof command === "string") return command;
    if (Array.isArray(command)) {
      return command
        .map(String)
        .map((word) =>
          /[\s'"\\;&|]/.test(word)
            ? `'${word.replaceAll("'", "'\\''")}'`
            : word,
        )
        .join(" ");
    }
  }
  return toolCall.title ?? "";
}

function pathsOf(toolCall: ToolCall): string[] {
  return (toolCall.locations ?? [])
    .map((location) => location.path)
    .filter((p): p is string => typeof p === "string" && p !== "");
}

function real(target: string): string {
  try {
    return realpathSync(target);
  } catch {
    return target;
  }
}

function inside(root: string, target: string): boolean {
  const relative = path.relative(root, target);
  return (
    relative === "" ||
    (!relative.startsWith("..") && !path.isAbsolute(relative))
  );
}

function insideWritable(cwd: string, target: string): boolean {
  const resolved = real(target);
  return (
    inside(real(cwd), resolved) ||
    inside(cwd, resolved) ||
    inside("/tmp", resolved) ||
    inside(real("/tmp"), resolved)
  );
}

function home(): string {
  return process.env.HOME || os.homedir() || "/root";
}

function expandHome(word: string): string {
  return word
    .replace(/^~(?=\/|$)/, home())
    .replace(/^\$\{?HOME\}?(?=\/|$)/, home());
}

// rmOutside lists the targets of a recursive or forced rm that resolve (from
// the directory the command has cd'd to, through symlinks that exist) outside
// the checkout and /tmp.
function rmOutside(command: string, cwd: string): string[] {
  let current = cwd;
  const out: string[] = [];
  for (const segment of segmentsOf(command)) {
    if (segment.argv0 === "cd") {
      const to = segment.args.find((arg) => !arg.startsWith("-"));
      current = to ? path.resolve(current, expandHome(to)) : home();
      continue;
    }
    if (segment.argv0 !== "rm") continue;
    const flags = segment.args.filter((arg) => arg.startsWith("-"));
    if (
      !flags.some(
        (arg) =>
          /^-[a-zA-Z]*[rRf]/.test(arg) ||
          arg === "--recursive" ||
          arg === "--force",
      )
    )
      continue;
    for (const arg of segment.args) {
      if (arg.startsWith("-")) continue;
      const target = path.resolve(current, expandHome(arg));
      if (!insideWritable(cwd, target)) out.push(arg);
    }
  }
  return out;
}

const readOnlyKinds = new Set(["read", "search", "think"]);

// sessionRuleFor is what "allow for this session" remembers in all mode: the
// program and its subcommand (git push, npm test), not every execute call.
function sessionRuleFor(command: string): string {
  const first = segmentsOf(command)[0];
  if (!first) return "all:execute";
  const sub =
    first.argv0 === "git"
      ? gitSubcommand(first.args).sub
      : firstVerb(first.args);
  return `all:execute:${[first.argv0, sub].filter(Boolean).join(" ")}`;
}

// gateFor applies the session's policy to one permission request. undefined
// means grant it without asking.
export function gateFor(
  toolCall: ToolCall,
  mode: ApprovalMode,
  cwd: string,
): Gate | undefined {
  if (mode === "autonomous" || isOwnTool(toolCall)) return undefined;
  const kind = toolCall.kind ?? "other";
  const title = (toolCall.title ?? "").trim() || kind;
  if (kind === "execute") {
    const command = commandOf(toolCall);
    if (command.length > maxMatchedCommandChars) {
      return {
        rule: "command-too-long",
        title: `Run (a very long command): ${oneLine(command, 200)}`,
        detail: {
          toolKind: kind,
          command: command.slice(0, maxMatchedCommandChars),
          rule: "command-too-long",
          policyVersion: approvalPolicyVersion,
        },
      };
    }
    const context: PolicyContext = {
      command,
      segments: segmentsOf(command),
      pipelines: pipelinesOf(command),
      cwd,
    };
    for (const rule of riskyCommands) {
      const hit = rule.matches(context);
      if (hit === false || (Array.isArray(hit) && hit.length === 0)) continue;
      return {
        rule: rule.id,
        title: `Run (${rule.label}): ${oneLine(command)}`,
        detail: {
          toolKind: kind,
          command,
          ...(Array.isArray(hit) ? { paths: hit } : {}),
          rule: rule.id,
          policyVersion: approvalPolicyVersion,
        },
      };
    }
  }
  if (kind === "edit" || kind === "delete" || kind === "move") {
    const outside = pathsOf(toolCall).filter(
      (p) => !insideWritable(cwd, path.resolve(cwd, p)),
    );
    if (outside.length > 0) {
      return {
        rule: "write-outside-checkout",
        title: `${title} (outside the checkout)`,
        detail: {
          toolKind: kind,
          paths: outside,
          rule: "write-outside-checkout",
          policyVersion: approvalPolicyVersion,
        },
      };
    }
  }
  if (mode === "all" && !readOnlyKinds.has(kind)) {
    const command = kind === "execute" ? commandOf(toolCall) : undefined;
    const rule =
      command !== undefined ? sessionRuleFor(command) : `all:${kind}`;
    return {
      rule,
      title: command ? `Run: ${oneLine(command)}` : title,
      detail: {
        toolKind: kind,
        ...(command ? { command } : {}),
        paths: pathsOf(toolCall),
        rule,
        policyVersion: approvalPolicyVersion,
      },
    };
  }
  return undefined;
}

// oneLine flattens text to one line of at most max characters.
export function oneLine(text: string, max = 400): string {
  const flat = text.replace(/\s+/g, " ").trim();
  return flat.length > max ? `${flat.slice(0, max - 1)}…` : flat;
}

// The decision clock: how long this turn has waited on people, so the turn
// timeout counts only the agent's own time. Every decision (the driver's
// permission requests and the bex MCP server's ask_user, possibly several at
// once) writes its own entry file in one directory next to the status file,
// so no two writers share a file. The waited time is the union of the
// entries' intervals: parallel waits count once.
export interface DecisionClock {
  pending: boolean;
  waitedMs: number;
}

interface ClockEntry {
  start: number;
  end?: number;
}

// unionMs is the total length of the intervals' union; an open one runs to now.
export function unionMs(entries: ClockEntry[], now = Date.now()): number {
  const spans = entries
    .map((entry) => [entry.start, entry.end ?? now] as const)
    .filter(
      ([start, end]) =>
        Number.isFinite(start) && Number.isFinite(end) && end > start,
    )
    .sort((a, b) => a[0] - b[0]);
  let total = 0;
  let open = -Infinity;
  let close = -Infinity;
  for (const [start, end] of spans) {
    if (start > close) {
      if (close > open) total += close - open;
      [open, close] = [start, end];
    } else if (end > close) {
      close = end;
    }
  }
  if (close > open) total += close - open;
  return total;
}

export async function readDecisionClock(
  clockDir: string,
  now = Date.now(),
): Promise<DecisionClock> {
  if (!clockDir) return { pending: false, waitedMs: 0 };
  let names: string[];
  try {
    names = await readdir(clockDir);
  } catch {
    return { pending: false, waitedMs: 0 };
  }
  const entries: ClockEntry[] = [];
  for (const name of names.filter((n) => n.endsWith(".json"))) {
    try {
      const raw = JSON.parse(
        await readFile(path.join(clockDir, name), "utf8"),
      ) as Record<string, unknown>;
      if (typeof raw.start !== "number") continue;
      entries.push({
        start: raw.start,
        ...(typeof raw.end === "number" ? { end: raw.end } : {}),
      });
    } catch {
      // A torn or foreign file is not a wait.
    }
  }
  return {
    pending: entries.some((entry) => entry.end === undefined),
    waitedMs: unionMs(entries, now),
  };
}

// resetDecisionClock clears the clock at the start of a turn.
export async function resetDecisionClock(clockDir: string): Promise<void> {
  if (clockDir)
    await rm(clockDir, { recursive: true, force: true }).catch(() => undefined);
}

async function clockEntry(
  clockDir: string,
  id: string,
  entry: ClockEntry,
): Promise<void> {
  if (!clockDir) return;
  await mkdir(clockDir, { recursive: true });
  await writeJsonAtomic(path.join(clockDir, `${id}.json`), entry);
}

export interface DecisionAnswer {
  state: string;
  allow?: boolean;
  scope?: string;
  text?: string;
}

export interface DecisionRequest {
  kind: "approval" | "question";
  title: string;
  detail: Record<string, unknown>;
}

const openTimeoutMs = 60_000;
// The gateway's await is held up to 20 s at bex-api; the client waits longer.
const awaitTimeoutMs = 45_000;
const retryDelayMs = 5_000;
// decisionTtlMs is the decision's own lifetime (ADR100 D2): no answer can
// arrive after it, so retrying an unreachable gateway longer is pointless.
export const decisionTtlMs = 24 * 60 * 60_000;

function answerOf(payload: unknown): DecisionAnswer | undefined {
  if (!payload || typeof payload !== "object") return undefined;
  const { state, answer } = payload as Record<string, unknown>;
  if (typeof state !== "string") return undefined;
  const out: DecisionAnswer = { state };
  if (answer && typeof answer === "object") {
    const a = answer as Record<string, unknown>;
    if (typeof a.allow === "boolean") out.allow = a.allow;
    if (typeof a.scope === "string") out.scope = a.scope;
    if (typeof a.text === "string") out.text = a.text;
  }
  return out;
}

// scrubValue applies scrub to every string in a JSON value (keys included),
// so a secret is redacted as text rather than in its JSON-escaped form.
function scrubValue(value: unknown, scrub: (text: string) => string): unknown {
  if (typeof value === "string") return scrub(value);
  if (Array.isArray(value)) return value.map((item) => scrubValue(item, scrub));
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [
        scrub(key),
        scrubValue(item, scrub),
      ]),
    );
  }
  return value;
}

type FinalOrRetry =
  { reply: GatewayReply & { ok: true } } | { stop: DecisionAnswer };

// postUntilFinal posts body until the gateway answers, retrying unavailable and
// rate-limited replies every few seconds until deadline. A refusal, an abort
// or the deadline ends it with a canceled or expired answer.
async function postUntilFinal(
  url: string,
  body: string,
  timeoutMs: number,
  deadline: number,
  signal?: AbortSignal,
): Promise<FinalOrRetry> {
  for (;;) {
    if (signal?.aborted) return { stop: { state: "canceled" } };
    if (Date.now() >= deadline) return { stop: { state: "expired" } };
    const reply = await postToGateway(
      url,
      { headers: { "content-type": "application/json" }, body },
      { label: "decision endpoint", fallbackCode: "unavailable", timeoutMs },
    );
    if (reply.ok) return { reply };
    if (
      reply.error.code !== "unavailable" &&
      reply.error.code !== "rate_limited"
    ) {
      process.stderr.write(
        `bex approval: decision refused: ${reply.error.code}\n`,
      );
      return { stop: { state: "canceled" } };
    }
    await sleep(retryDelayMs, undefined, { signal, ref: false }).catch(
      () => undefined,
    );
  }
}

// requestDecision opens one decision and waits for its final state. A
// transient gateway failure is retried until the decision's own lifetime
// ends; a refusal (the turn ended, the session's Pod changed) or an abort ends
// the wait as canceled. It never throws. Every string is scrubbed of session
// secrets before it leaves the sandbox. The wait counts on the decision clock
// from the moment the decision is open, so retrying an unreachable gateway is
// the agent's time, not a person's.
export async function requestDecision(
  endpoint: string,
  request: DecisionRequest,
  options: {
    clockPath: string;
    scrub?: (text: string) => string;
    signal?: AbortSignal;
    ttlMs?: number;
  },
): Promise<DecisionAnswer> {
  const scrub = options.scrub ?? ((text: string) => text);
  const requestId = randomUUID();
  const deadline = Date.now() + (options.ttlMs ?? decisionTtlMs);
  let started: number | undefined;
  try {
    const body = JSON.stringify({
      requestId,
      kind: request.kind,
      title: scrub(request.title),
      detail: scrubValue(request.detail, scrub),
    });
    const opened = await postUntilFinal(
      endpoint,
      body,
      openTimeoutMs,
      deadline,
      options.signal,
    );
    if ("stop" in opened) return opened.stop;
    const payload = opened.reply.payload as Record<string, unknown> | undefined;
    const id = typeof payload?.id === "string" ? payload.id : "";
    const first = answerOf(payload);
    if (!id) return { state: "canceled" };
    if (first && first.state !== "pending") return first;
    started = Date.now();
    await clockEntry(options.clockPath, requestId, { start: started }).catch(
      () => undefined,
    );
    const awaitUrl = `${endpoint.replace(/\/+$/, "")}/${id}`;
    for (;;) {
      const awaited = await postUntilFinal(
        awaitUrl,
        "{}",
        awaitTimeoutMs,
        deadline,
        options.signal,
      );
      if ("stop" in awaited) return awaited.stop;
      const answer = answerOf(awaited.reply.payload);
      if (answer && answer.state !== "pending") return answer;
    }
  } catch (error) {
    process.stderr.write(
      `bex approval: decision failed: ${describeError(error)}\n`,
    );
    return { state: "canceled" };
  } finally {
    if (started !== undefined) {
      await clockEntry(options.clockPath, requestId, {
        start: started,
        end: Date.now(),
      }).catch(() => undefined);
    }
  }
}

type PermissionOption = RequestPermissionRequest["options"][number];

function pick(
  options: PermissionOption[],
  kinds: string[],
): PermissionOption | undefined {
  for (const kind of kinds) {
    const option = options.find((candidate) => candidate.kind === kind);
    if (option) return option;
  }
  return undefined;
}

// grant selects the engine's allow option. "always" is the autonomous
// driver's answer and a member's "for this session"; "once" is every other
// grant in an approval mode, because an engine persists an allow_always as a
// rule (Claude Code: a command prefix) that would silence a later ask.
export function grant(
  request: RequestPermissionRequest,
  scope: "once" | "always" = "always",
): RequestPermissionResponse {
  const option =
    pick(
      request.options,
      scope === "once"
        ? ["allow_once", "allow_always"]
        : ["allow_always", "allow_once"],
    ) ??
    request.options.find((candidate) => candidate.kind.startsWith("allow")) ??
    request.options[0];
  if (!option) return { outcome: { outcome: "cancelled" } };
  return { outcome: { outcome: "selected", optionId: option.optionId } };
}

function refuse(request: RequestPermissionRequest): RequestPermissionResponse {
  const option = pick(request.options, ["reject_once", "reject_always"]);
  if (!option) return { outcome: { outcome: "cancelled" } };
  return { outcome: { outcome: "selected", optionId: option.optionId } };
}

export interface PermissionContext {
  mode: ApprovalMode;
  decisionUrl: string;
  cwd: string;
  clockPath: string;
  scrub?: (text: string) => string;
  signal?: AbortSignal;
  decide?: typeof requestDecision;
}

// createPermissionHandler is the ACP client's session/request_permission
// handler for one turn. A gated call a member allowed "for this session" is
// remembered by its rule for the rest of the turn, and granted allow_always so
// the engine also remembers it; everything else in an approval mode is
// granted once.
export function createPermissionHandler(context: PermissionContext) {
  const allowedRules = new Set<string>();
  const decide = context.decide ?? requestDecision;
  return async (
    request: RequestPermissionRequest,
  ): Promise<RequestPermissionResponse> => {
    if (!context.decisionUrl || context.mode === "autonomous")
      return grant(request);
    const gate = gateFor(request.toolCall, context.mode, context.cwd);
    if (!gate) return grant(request, "once");
    if (allowedRules.has(gate.rule)) return grant(request, "always");
    const answer = await decide(
      context.decisionUrl,
      { kind: "approval", title: gate.title, detail: gate.detail },
      {
        clockPath: context.clockPath,
        scrub: context.scrub,
        signal: context.signal,
      },
    );
    if (answer.state === "allowed" && answer.allow !== false) {
      if (answer.scope === "session") {
        allowedRules.add(gate.rule);
        return grant(request, "always");
      }
      return grant(request, "once");
    }
    return refuse(request);
  };
}
