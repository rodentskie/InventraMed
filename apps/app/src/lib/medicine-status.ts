export type MedicineStatus = "green" | "yellow" | "red"

const MS_PER_DAY = 1000 * 60 * 60 * 24

function startOfDayUTC(date: Date): number {
  return Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate())
}

// `expirationDate` is a YYYY-MM-DD date string, as returned by the API.
// `thresholdDays` is `warning_threshold_days` from GET /settings.
export function getMedicineStatus(
  expirationDate: string,
  thresholdDays: number,
  now: Date = new Date(),
): MedicineStatus {
  const expiresAt = startOfDayUTC(new Date(`${expirationDate}T00:00:00Z`))
  const today = startOfDayUTC(now)
  const daysUntilExpiration = Math.floor((expiresAt - today) / MS_PER_DAY)

  if (daysUntilExpiration < 0) return "red"
  if (daysUntilExpiration <= thresholdDays) return "yellow"
  return "green"
}
