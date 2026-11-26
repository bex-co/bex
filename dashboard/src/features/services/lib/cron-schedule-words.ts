// What a cron job's header says after "Runs <command>": its schedule in
// Render's words (w3/m643). Render's header (`AuthContext-*.js` `z`) gates the
// schedule, then words it with cronstrue's defaults, lowers the first letter
// and writes " UTC" after each AM or PM. Its own module, apart from cron.ts,
// so cronstrue and its locales load with the service header's chunk only, not
// with the create form's and Settings' (which import cron.ts).

import cronstrue from "cronstrue";
// Each locale registers itself on cronstrue when imported. Only the languages
// the dashboard speaks, imported statically, so the server and the browser
// word a schedule alike on the first render.
import "cronstrue/locales/zh_CN.js";
import "cronstrue/locales/ko.js";
import "cronstrue/locales/ja.js";
import "cronstrue/locales/es.js";
import "cronstrue/locales/pt_BR.js";
import "cronstrue/locales/ru.js";
import "cronstrue/locales/de.js";
import { dashboardLanguage } from "@/common/lib/format/language";
import { LANGUAGE_TAGS, type SupportedLanguage } from "@/i18n/config";
import { isValidCron } from "@/features/services/lib/cron";

/** cronstrue's locale for each dashboard language. */
const CRONSTRUE_LOCALE: Record<SupportedLanguage, string> = {
  en: "en",
  zh: "zh_CN",
  ko: "ko",
  ja: "ja",
  es: "es",
  pt: "pt_BR",
  ru: "ru",
  de: "de",
};

// A time of day as cronstrue writes one: "09:00 AM" in English, "上午 09:00"
// in zh and ko (a 12-hour clock with its period first), "09:00" in the
// languages on a 24-hour clock.
const CLOCK = /(?:(?:上午|下午|오전|오후) )?\d{2}:\d{2}(?: [AP]M)?/g;

// Where a language names the zone: after the time, as Render does in English
// ("09:00 AM UTC") and the header's es/pt/ru/de phrases did ("um 09:00 UTC");
// before it in zh, ko and ja, as their phrases did ("每天 UTC 09:00").
const UTC_FIRST: ReadonlySet<SupportedLanguage> = new Set(["zh", "ko", "ja"]);

// The one phrase cronstrue's German opens with a noun ("Minuten 1 bis 5"),
// which keeps its capital when the first letter is lowered.
const GERMAN_NOUN_FIRST = /^Minuten\b/;

/**
 * The forms robfig's parser takes and Render's gate (cron-validator) refuses,
 * where Render's header reads nothing and cronstrue would misread them ("at
 * +5 minutes past the hour", "minutes * through 5 past the hour"): a sign, a
 * range that starts at a star and an empty list item.
 */
function renderRefuses(fields: string[]): boolean {
  return fields.some((field) => {
    const starred = field.replace(/\?/g, "*");
    return (
      starred.includes("+") ||
      starred.includes("*-") ||
      starred.split(",").includes("")
    );
  });
}

function withUtc(words: string, lang: SupportedLanguage): string {
  if (!UTC_FIRST.has(lang)) return words.replace(CLOCK, "$& UTC");
  return words.replace(CLOCK, (clock: string, at: number) =>
    // cronstrue sets a time against its words with no space in some phrases
    // ("在上午 09:00", "次において実施09:00"): the zone brings its own.
    at > 0 && !/\s/.test(words[at - 1]) ? ` UTC ${clock}` : `UTC ${clock}`,
  );
}

/**
 * A cron job's schedule as Render's header words it: cronstrue's phrase in
 * the dashboard language, its first letter lowered and the zone with each time
 * ("every 40 minutes", "at 09:00 AM UTC, Monday through Friday"). Null where
 * Render's header reads nothing — a schedule bex-api would refuse or one that
 * never fires (`isValidCron`), a form Render's gate refuses, or one cronstrue
 * cannot word — and the header then shows nothing after the command.
 */
export function describeCronRun(
  schedule: string,
  language: string | undefined,
): string | null {
  const fields = schedule.trim().split(/\s+/);
  if (!isValidCron(schedule) || renderRefuses(fields)) return null;
  const lang = dashboardLanguage(language);
  let words: string;
  try {
    words = cronstrue.toString(fields.join(" "), {
      locale: CRONSTRUE_LOCALE[lang],
    });
  } catch {
    return null;
  }
  if (!(lang === "de" && GERMAN_NOUN_FIRST.test(words))) {
    words =
      words.charAt(0).toLocaleLowerCase(LANGUAGE_TAGS[lang]) + words.slice(1);
  }
  return withUtc(words, lang);
}
