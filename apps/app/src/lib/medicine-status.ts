export type MedicineStatus = "green" | "yellow" | "red"

// Mirrors the DB default for `settings.warning_threshold_days`
// (apps/migration/transactions/00008_create_settings_table.sql). There is no
// `/settings` endpoint yet, so this can't be read from the API.
const WARNING_THRESHOLD_DAYS = 30

const MS_PER_DAY = 1000 * 60 * 60 * 24

function startOfDayUTC(date: Date): number {
  return Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate())
}

// `expirationDate` is a YYYY-MM-DD date string, as returned by the API.
export function getMedicineStatus(
  expirationDate: string,
  now: Date = new Date(),
): MedicineStatus {
  const expiresAt = startOfDayUTC(new Date(`${expirationDate}T00:00:00Z`))
  const today = startOfDayUTC(now)
  const daysUntilExpiration = Math.floor((expiresAt - today) / MS_PER_DAY)

  if (daysUntilExpiration < 0) return "red"
  if (daysUntilExpiration <= WARNING_THRESHOLD_DAYS) return "yellow"
  return "green"
}
