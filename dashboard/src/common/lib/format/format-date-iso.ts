import { parseISO, isValid, format } from "date-fns";

/**
 * Format date as ISO YYYY-MM-DD
 * @param dateString - Date string to format
 * @returns Formatted date in YYYY-MM-DD format or null if invalid
 */
export const formatDateISO = (
  dateString: string | null | undefined,
): string | null => {
  if (!dateString) {
    return null;
  }

  // Strip time portion so ISO timestamps are treated as the plain calendar date
  const datePart = dateString.includes("T")
    ? dateString.split("T")[0]
    : dateString;
  const date = parseISO(datePart);

  if (!isValid(date)) {
    return null;
  }

  return format(date, "yyyy-MM-dd");
};
