import { Fragment } from "react";
import { Linking, Text, type TextStyle } from "react-native";
import { useTheme } from "@/common/theme";

type Token =
  { kind: "text"; text: string } | { kind: "link"; text: string; url: string };

/** Native-safe Markdown subset: text plus validated HTTPS links, never HTML/images. */
export function SafeMarkdownText({
  value,
  style,
}: {
  value: string;
  style?: TextStyle;
}) {
  const theme = useTheme().colorTheme;
  return (
    <Text style={[{ color: theme.foreground }, style]}>
      {tokenizeSafeMarkdown(value).map((token, index) =>
        token.kind === "link" ? (
          <Text
            key={`${token.url}-${index}`}
            accessibilityRole="link"
            style={{ color: theme.primary, textDecorationLine: "underline" }}
            onPress={() => void Linking.openURL(token.url)}
          >
            {token.text}
          </Text>
        ) : (
          <Fragment key={index}>{token.text}</Fragment>
        ),
      )}
    </Text>
  );
}

export function tokenizeSafeMarkdown(value: string): Token[] {
  const tokens: Token[] = [];
  const pattern = /(!?)\[([^\]]*)\]\(([^)]+)\)/g;
  let cursor = 0;
  for (const match of value.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > cursor) {
      tokens.push({ kind: "text", text: value.slice(cursor, index) });
    }
    const isImage = match[1] === "!";
    const label = match[2] || match[3];
    const url = safeHttpsUrl(match[3]);
    if (isImage) {
      tokens.push({ kind: "text", text: label ? `[${label}]` : "[image]" });
    } else if (url) {
      tokens.push({ kind: "link", text: label, url });
    } else {
      tokens.push({ kind: "text", text: label });
    }
    cursor = index + match[0].length;
  }
  if (cursor < value.length) {
    tokens.push({ kind: "text", text: value.slice(cursor) });
  }
  return tokens.length > 0 ? tokens : [{ kind: "text", text: value }];
}

function safeHttpsUrl(raw: string): string | null {
  try {
    const url = new URL(raw);
    if (url.protocol !== "https:" || url.username || url.password) return null;
    return url.toString();
  } catch {
    return null;
  }
}
