import type { ComponentProps } from "react";
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";

/**
 * Render's Account Settings columns (Name, Permissions, Key, Created, Last
 * used, then the trash icon), with the key's client ID in its Key column
 * (w9/132). No Created by column: the default list is the caller's own keys,
 * and an Admin's every-member view names each key's creator under its name.
 * The table lays out fixed, so its loading frame has the ready table's
 * columns whatever the keys hold. Name takes what the others leave: 162 px of
 * the card's 654 at a 1,280 px window, so the revoke button stays inside the
 * card; the Client ID shows its first characters, as Render's key prefix.
 */
const COLUMNS = [
  { label: "apiKeys.colName", width: undefined },
  { label: "apiKeys.colPermissions", width: "w-36" },
  { label: "apiKeys.colClientId", width: "w-34" },
  { label: "apiKeys.colCreated", width: "w-18" },
  { label: "apiKeys.colLastUsed", width: "w-22" },
] as const;
const ACTIONS_WIDTH = "w-13";

// A held cell covers the columns scrolling under it, so it is opaque: the
// card's colour, or the row's hover tint over it.
const HELD_CELL =
  "z-10 bg-card [tr:hover>&]:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-card))]";

/**
 * The Name cell holds still at the table's left edge, and the revoke cell
 * below at its right, while the columns between them scroll in a card too
 * narrow for the table (w9/132: a 390 px window).
 */
export const API_KEYS_NAME_CELL = `sticky left-0 ${HELD_CELL}`;
export const API_KEYS_ACTIONS_CELL = `sticky right-0 text-right ${HELD_CELL}`;

/**
 * The API keys table (w9/132), with the rows passed as children. The panel's
 * keys and its loading frame both draw it, so the two have the same column
 * boxes and header row. Below 616 px (Name at 124) the table scrolls in its
 * card instead of crushing Name.
 */
export function ApiKeysTable({ children, ...props }: ComponentProps<"table">) {
  const { t } = useTranslations();
  return (
    <Table className="min-w-154 table-fixed" {...props}>
      <colgroup>
        {COLUMNS.map(({ label, width }) => (
          <col key={label} className={width} />
        ))}
        <col className={ACTIONS_WIDTH} />
      </colgroup>
      <TableHeader>
        <TableRow>
          {/* A label longer than its column (ru "Последнее использование")
              breaks at a hyphenation point, not mid-syllable. */}
          {COLUMNS.map(({ label }, i) => (
            <TableHead
              key={label}
              className={cn("hyphens-auto", i === 0 && API_KEYS_NAME_CELL)}
            >
              {t(label)}
            </TableHead>
          ))}
          {/* An in-flow head, so the held column has a header cell. */}
          <TableHead className={API_KEYS_ACTIONS_CELL}>
            <span className="sr-only">{t("common.colActions")}</span>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>{children}</TableBody>
    </Table>
  );
}
