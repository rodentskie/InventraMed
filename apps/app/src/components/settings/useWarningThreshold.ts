"use client"

import { toaster } from "@inventramed/snippets/toaster"
import { useEffect, useState } from "react"
import { getSettings } from "../../actions/settings"

// Fetches the expiration warning threshold from the settings once on mount.
// It is null while loading and when the fetch fails, which shows an error
// toast; callers then have no status to show.
export function useWarningThreshold(): number | null {
  const [thresholdDays, setThresholdDays] = useState<number | null>(null)

  useEffect(() => {
    let active = true

    getSettings().then((result) => {
      if (!active) return
      if (!result.success || !result.data) {
        toaster.create({
          type: "error",
          title: "Couldn't load the settings",
          description: "Expiration statuses can't be shown.",
        })
        return
      }
      setThresholdDays(result.data.warning_threshold_days)
    })

    return () => {
      active = false
    }
  }, [])

  return thresholdDays
}
