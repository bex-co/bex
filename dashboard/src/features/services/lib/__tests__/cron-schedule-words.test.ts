import { describe, expect, it } from "vitest";

import { describeCronRun } from "@/features/services/lib/cron-schedule-words";

// w3/m643: a cron job's header words its schedule as Render's does
// (`AuthContext-*.js` `z`): cronstrue 2.50.0's phrase with its defaults, the
// first letter lowered and " UTC" after each AM or PM. The first eight rows
// are the phrases w3/m555 read from Render's bundle
// (docs/render-artifacts/cron-dashboard.md, "The facts under the source").
describe("describeCronRun", () => {
  const en = (s: string) => describeCronRun(s, "en");

  it.each([
    ["0 9 * * *", "at 09:00 AM UTC"],
    ["* * * * *", "every minute"],
    ["*/2 * * * *", "every 2 minutes"],
    ["0 * * * *", "every hour"],
    ["5 * * * *", "at 5 minutes past the hour"],
    ["0 */2 * * *", "at 0 minutes past the hour, every 2 hours"],
    ["0 9 * * 1", "at 09:00 AM UTC, only on Monday"],
    ["0 9 1 * *", "at 09:00 AM UTC, on day 1 of the month"],
    // What bex named no shape for, and showed as its expression.
    ["0 9 * * 1-5", "at 09:00 AM UTC, Monday through Friday"],
    ["*/40 * * * *", "every 40 minutes"],
    ["0 0 1 * *", "at 12:00 AM UTC, on day 1 of the month"],
    ["0 9,17 * * *", "at 09:00 AM UTC and 05:00 PM UTC"],
    [
      "0 9-17 * * 1-5",
      "every hour, between 09:00 AM UTC and 05:00 PM UTC, Monday through Friday",
    ],
    ["0 0 * * MON", "at 12:00 AM UTC, only on Monday"],
    ["0 0 ? * *", "at 12:00 AM UTC"],
    ["30 13 * * *", "at 01:30 PM UTC"],
  ])("words %s as %s", (schedule, words) => {
    expect(en(schedule)).toBe(words);
  });

  it("words a schedule bex-api accepts with extra spaces", () => {
    expect(en("  0   9 * * * ")).toBe("at 09:00 AM UTC");
  });

  // Render reads nothing for a schedule its gate refuses: bex reads nothing
  // for one bex-api refuses, one that never fires, or a form robfig takes
  // that Render's cron-validator refuses (cronstrue misreads those).
  it.each([
    ["not a cron"],
    ["99 99 * * *"],
    ["0 0 * * 7"],
    ["0 0 * * *  *"],
    ["0 0 31 2 *"],
    ["+5 * * * *"],
    ["*/+2 * * * *"],
    ["*-5 * * * *"],
    ["1,,2 * * * *"],
    [",5 * * * *"],
    [""],
  ])("reads nothing for %j", (schedule) => {
    expect(en(schedule)).toBeNull();
  });

  // Every dashboard language has its cronstrue locale, the zone where its
  // own phrases put it: before the time in zh, ko and ja, after it elsewhere.
  it.each([
    ["en-US", "at 09:00 AM UTC, Monday through Friday"],
    ["zh", "在 UTC 上午 09:00, 星期一至星期五"],
    ["ko", "시간 UTC 오전 09:00, 월요일에서 금요일까지"],
    ["ja", "次において実施 UTC 09:00、月曜日 から 金曜日 まで"],
    ["es", "a las 09:00 UTC, de lunes a viernes"],
    ["pt-BR", "às 09:00 UTC, de segunda-feira a sexta-feira"],
    ["ru", "в 09:00 UTC, с понедельника по пятницу"],
    ["de", "um 09:00 UTC, Montag bis Freitag"],
  ])("words 0 9 * * 1-5 in %s", (language, words) => {
    expect(describeCronRun("0 9 * * 1-5", language)).toBe(words);
  });

  it.each([
    ["*/40 * * * *", "每隔 40 分钟"],
    [
      "0 9-17 * * 1-5",
      "每小时, 在 UTC 上午 09:00 和 UTC 下午 05:00 之间, 星期一至星期五",
    ],
    ["0 0 1 * *", "在 UTC 上午 12:00, 限每月 1 号"],
  ])("words %s in zh", (schedule, words) => {
    expect(describeCronRun(schedule, "zh")).toBe(words);
  });

  it.each([
    ["*/40 * * * *", "alle 40 Minuten"],
    ["0 9 * * 1", "um 09:00 UTC, nur jeden Montag"],
    // A German noun keeps its capital.
    ["1-5 * * * *", "Minuten 1 bis 5"],
  ])("words %s in de", (schedule, words) => {
    expect(describeCronRun(schedule, "de")).toBe(words);
  });
});
