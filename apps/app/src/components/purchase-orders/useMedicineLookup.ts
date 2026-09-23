"use client"

import { useEffect, useMemo, useState } from "react"
import { listMedicines } from "../../actions/medicines"
import type { Medicine } from "../../types/medicine"

// The API's max page size (`limit` maxes out at 100). This is a best-effort
// lookup for resolving medicine_id -> name: it only covers the first 100
// active medicines and excludes deleted ones. An item referencing a medicine
// outside that set falls back to showing its raw medicine_id.
const MEDICINE_LOOKUP_LIMIT = 100

export function useMedicineLookup(): Map<string, Medicine> {
  const [medicines, setMedicines] = useState<Medicine[]>([])

  useEffect(() => {
    listMedicines({ limit: MEDICINE_LOOKUP_LIMIT, offset: 0 }).then((result) => {
      if (result.success && result.data) setMedicines(result.data.data)
    })
  }, [])

  return useMemo(
    () => new Map(medicines.map((medicine) => [medicine.id, medicine])),
    [medicines],
  )
}
