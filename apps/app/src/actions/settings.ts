"use server"

import type { ActionResult } from "../types/action"
import type { ErrorResponse } from "../types/medicine"
import type { Settings } from "../types/settings"

const GENERIC_ERROR = "Something went wrong. Please try again."
// These statuses carry a meaningful `error` message from the API; others
// (e.g. 500) may not always return JSON, so they fall back to a generic one.
const MESSAGE_STATUSES = [404]

// Public endpoint: the scanner page calls this without a logged-in session,
// so no auth headers are sent.
export async function getSettings(): Promise<ActionResult<Settings>> {
  try {
    const apiPrefix = process.env.API_PREFIX ?? ""
    const res = await fetch(`${process.env.API_URL}${apiPrefix}/settings`, {
      cache: "no-store",
    })
    if (!res.ok) {
      const error = MESSAGE_STATUSES.includes(res.status)
        ? ((await res.json()) as ErrorResponse).error || GENERIC_ERROR
        : GENERIC_ERROR
      return { success: false, data: null, error }
    }

    const body = (await res.json()) as { data: Settings }
    return { success: true, data: body.data, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}
