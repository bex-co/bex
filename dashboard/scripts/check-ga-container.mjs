// Checks the live Google tag container against what the dashboard's analytics
// privacy rule depends on (AGENTS.md § Analytics). The container is edited in
// Tag Manager and the GA admin, outside this repository, and its defaults send
// the raw URL and title of every page: run this after any change there.
//
//   yarn check:ga-container            # the production stream
//   yarn check:ga-container G-XXXXXXX  # another stream

import { pathToFileURL } from "node:url";

const PRODUCTION_ID = "G-7X5XTVZ55N";
export const READY_EVENT = "bex_ga_ready";

/** The container's `resource` object out of gtag.js, or null when the script
 * no longer has that shape. */
export function containerResource(js) {
  const start = js.indexOf('"resource":');
  const end = js.indexOf(',"runtime"', start);
  if (start < 0 || end < 0) return null;
  try {
    return JSON.parse(js.slice(start + '"resource":'.length, end));
  } catch {
    return null;
  }
}

/** Every way the container breaks the rule; empty when it holds. */
export function containerProblems(resource) {
  const { tags = [], predicates = [], rules = [] } = resource;
  const problems = [];

  const ga = tags.findIndex((tag) => tag.function === "__dest_ga");
  if (ga < 0) return ["the container has no Google Analytics tag"];

  // The tag must fire on the dashboard's ready event and on nothing else:
  // fired at initialization it reports the raw URL and title.
  const firesOn = rules
    .filter(([, add]) => add?.[0] === "add" && add.slice(1).includes(ga))
    .flatMap(([when]) => when.slice(1).map((index) => predicates[index]?.arg1));
  if (firesOn.length !== 1 || firesOn[0] !== READY_EVENT) {
    problems.push(
      `the Google Analytics tag fires on [${firesOn.join(", ")}], not only on ${READY_EVENT}`,
    );
  }

  const settings = JSON.stringify(tags[ga].vtp_configSettingsTable ?? []);
  if (!settings.includes('"send_page_view","parameterValue","false"')) {
    problems.push("the Google Analytics tag does not set send_page_view=false");
  }

  // Each automatic event reads the raw URL or the page's content.
  const detection = tags.find((tag) => tag.function === "__ogt_auto_events");
  for (const [key, value] of Object.entries(detection ?? {})) {
    const name = /^vtp_enable(.+)$/.exec(key)?.[1];
    if (name && name !== "InPageEventsHidden" && value === true) {
      problems.push(`automatic event detection is on: ${name}`);
    }
  }

  const userData = tags.find((tag) => tag.function === "__ogt_1p_data_v2");
  if (userData?.vtp_isEnabled === true) {
    problems.push("user-provided data detection is on");
  }
  return problems;
}

async function main(id) {
  const response = await fetch(
    `https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(id)}`,
  );
  if (!response.ok)
    throw new Error(`gtag.js for ${id}: HTTP ${response.status}`);
  const resource = containerResource(await response.text());
  if (!resource) throw new Error(`gtag.js for ${id}: no container resource`);
  const problems = containerProblems(resource);
  for (const problem of problems) console.error(`FAIL ${problem}`);
  if (problems.length) process.exit(1);
  console.log(`ok: container version ${resource.version} for ${id}`);
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  main(process.argv[2] || PRODUCTION_ID).catch((error) => {
    console.error(String(error.message ?? error));
    process.exit(1);
  });
}
