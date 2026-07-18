const VALID_ENV_KEY = /^[A-Za-z_][A-Za-z0-9_]*$/;

export const MAX_DOTENV_FILE_BYTES = 1024 * 1024;

export interface DotenvEntry {
  key: string;
  value: string;
  line: number;
}

export class DotenvParseError extends Error {
  constructor(
    public readonly line: number,
    public readonly reason: "assignment" | "key" | "quote" | "trailing",
  ) {
    super(`Invalid dotenv syntax on line ${line}`);
    this.name = "DotenvParseError";
  }
}

/**
 * Parse dotenv text as data only. There is no shell expansion, interpolation,
 * command substitution, or execution. Duplicate keys are deterministic: the
 * last assignment wins and carries its source line in the returned entry.
 */
export function parseDotenv(text: string): DotenvEntry[] {
  const entries = new Map<string, DotenvEntry>();
  const lines = text
    .replaceAll("\r\n", "\n")
    .replaceAll("\r", "\n")
    .split("\n");

  lines.forEach((source, index) => {
    const line = index + 1;
    let input = source.trim();
    if (!input || input.startsWith("#")) return;
    if (input.startsWith("export ") || input.startsWith("export\t")) {
      input = input.slice(6).trimStart();
    }
    const equals = input.indexOf("=");
    if (equals < 1) throw new DotenvParseError(line, "assignment");
    const key = input.slice(0, equals).trim();
    if (!VALID_ENV_KEY.test(key)) throw new DotenvParseError(line, "key");
    const value = parseValue(input.slice(equals + 1), line);
    entries.set(key, { key, value, line });
  });

  return [...entries.values()];
}

function parseValue(source: string, line: number): string {
  const input = source.trim();
  if (!input) return "";
  const quote = input[0];
  if (quote !== '"' && quote !== "'") return stripInlineComment(input);

  let value = "";
  let escaped = false;
  let close = -1;
  for (let index = 1; index < input.length; index += 1) {
    const character = input[index];
    if (quote === '"' && escaped) {
      value +=
        character === "n"
          ? "\n"
          : character === "r"
            ? "\r"
            : character === "t"
              ? "\t"
              : character;
      escaped = false;
      continue;
    }
    if (quote === '"' && character === "\\") {
      escaped = true;
      continue;
    }
    if (character === quote) {
      close = index;
      break;
    }
    value += character;
  }
  if (close < 0 || escaped) throw new DotenvParseError(line, "quote");
  const trailing = input.slice(close + 1).trim();
  if (trailing && !trailing.startsWith("#")) {
    throw new DotenvParseError(line, "trailing");
  }
  return value;
}

function stripInlineComment(value: string): string {
  const comment = value.search(/\s+#/);
  return (comment < 0 ? value : value.slice(0, comment)).trimEnd();
}
