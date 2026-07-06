import type { getClient } from "@/common/apollo/client";
import type { Session } from "@ory/client-fetch";

export type RouterContext = {
  client: ReturnType<typeof getClient>;
  session?: Session | null;
};
