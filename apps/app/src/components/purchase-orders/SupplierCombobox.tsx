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
import { listSuppliers } from "../../actions/suppliers"
import type { Supplier } from "../../types/supplier"

// The API's max page size: a search only ever needs to show this many
// matches at once, since the user keeps typing to narrow further.
const SEARCH_LIMIT = 20
const DEBOUNCE_MS = 300

interface SupplierComboboxProps {
  value: string
  onValueChange: (supplierId: string) => void
}

// Searches suppliers by name against the API as the user types, rather than
// relying on a bulk-fetched list — so it still finds a match once there are
// more suppliers than fit on one page.
export function SupplierCombobox({ value, onValueChange }: SupplierComboboxProps) {
  const [loading, setLoading] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  const { collection, set, reset } = useListCollection<Supplier>({
    initialItems: [],
    itemToString: (supplier) => supplier.name,
    itemToValue: (supplier) => supplier.id,
  })

  const search = useCallback(
    async (name: string) => {
      setLoading(true)
      const result = await listSuppliers({
        limit: SEARCH_LIMIT,
        offset: 0,
        name: name.trim() || undefined,
      })
      setLoading(false)
      if (result.success && result.data) set(result.data.data)
    },
    [set],
  )

  // Initial page of suppliers, so the list isn't empty before typing. Runs
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
        <ComboboxInput placeholder="Search supplier by name..." />
      </ComboboxControl>
      <ComboboxContent>
        <ComboboxEmpty>
          {loading ? <Spinner size="xs" /> : "No suppliers found"}
        </ComboboxEmpty>
        {collection.items.map((supplier) => (
          <ComboboxItem key={supplier.id} item={supplier}>
            <ComboboxItemText>{supplier.name}</ComboboxItemText>
          </ComboboxItem>
        ))}
      </ComboboxContent>
    </ComboboxRoot>
  )
}
