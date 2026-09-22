"use client"

import { Spinner, useListCollection } from "@chakra-ui/react"
import {
  ComboboxContent,
  ComboboxControl,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxItemText,
  ComboboxRoot,
} from "@inventramed/snippets/combobox"
import { useCallback, useEffect, useRef, useState } from "react"
import { listMedicines } from "../../actions/medicines"
import type { Medicine } from "../../types/medicine"

// The API's max page size: a search only ever needs to show this many
// matches at once, since the user keeps typing to narrow further.
const SEARCH_LIMIT = 20
const DEBOUNCE_MS = 300

function medicineLabel(medicine: Medicine): string {
  return `${medicine.name} (${medicine.barcode})`
}

interface MedicineComboboxProps {
  value: string
  onValueChange: (medicineId: string) => void
}

// Searches medicines by name against the API as the user types, rather than
// relying on a bulk-fetched list — so it still finds a match once there are
// more medicines than fit on one page.
export function MedicineCombobox({ value, onValueChange }: MedicineComboboxProps) {
  const [loading, setLoading] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  const { collection, set, reset } = useListCollection<Medicine>({
    initialItems: [],
    itemToString: medicineLabel,
    itemToValue: (medicine) => medicine.id,
  })

  const search = useCallback(
    async (name: string) => {
      setLoading(true)
      const result = await listMedicines({
        limit: SEARCH_LIMIT,
        offset: 0,
        name: name.trim() || undefined,
      })
      setLoading(false)
      if (result.success && result.data) set(result.data.data)
    },
    [set],
  )

  // Initial page of medicines, so the list isn't empty before typing. Runs
  // once on mount only: `search` is stable (it only closes over `set`).
  useEffect(() => {
    search("")
  }, [search])

  useEffect(() => {
    if (value === "") reset()
  }, [value, reset])

  const handleInputValueChange = (details: { inputValue: string }) => {
    clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => search(details.inputValue), DEBOUNCE_MS)
  }

  return (
    <ComboboxRoot
      collection={collection}
      value={value ? [value] : []}
      onValueChange={(details) => onValueChange(details.value[0] ?? "")}
      onInputValueChange={handleInputValueChange}
    >
      <ComboboxControl clearable>
        <ComboboxInput placeholder="Search medicine by name..." />
      </ComboboxControl>
      <ComboboxContent>
        <ComboboxEmpty>
          {loading ? (
            <Spinner size="xs" />
          ) : (
            "No medicines found"
          )}
        </ComboboxEmpty>
        {collection.items.map((medicine) => (
          <ComboboxItem key={medicine.id} item={medicine}>
            <ComboboxItemText>{medicineLabel(medicine)}</ComboboxItemText>
          </ComboboxItem>
        ))}
      </ComboboxContent>
    </ComboboxRoot>
  )
}
