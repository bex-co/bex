import { createLocalSetting } from "./local-setting";
import type { LogOrder } from "./log-rows";

/**
 * The reader's Options ▸ Order by, one for every explorer and kept across
 * visits in `localStorage`, as Render keeps its own direction
 * (`render-log-direction-setting`, w3/089). Ascending, the newest line at the
 * bottom, is the default, and a stored value it does not know reads as it.
 */
export const logOrderSetting = createLocalSetting<LogOrder>(
  "bex.logs.order",
  "asc",
  (raw) => (raw === "desc" ? "desc" : "asc"),
);
