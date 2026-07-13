/** Returns true if s is a 5-field cron expression (field syntax validated by the k8s CronJob controller). */
export function isValidCron(s: string): boolean {
  return s.trim().split(/\s+/).length === 5;
}
