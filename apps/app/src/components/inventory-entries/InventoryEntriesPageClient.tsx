"use client"

import { Button, Center, Flex, HStack, Heading, Spinner, Stack } from "@chakra-ui/react"
import { Alert } from "@inventramed/snippets/alert"
import {
  PaginationNextTrigger,
  PaginationPageText,
  PaginationPrevTrigger,
  PaginationRoot,
} from "@inventramed/snippets/pagination"
import { useCallback, useEffect, useMemo, useState } from "react"
import { listInventoryEntries } from "../../actions/inventory-entries"
import { listMedicines } from "../../actions/medicines"
import type { InventoryEntry } from "../../types/inventory-entry"
import type { Medicine } from "../../types/medicine"
import { InventoryEntriesTable } from "./InventoryEntriesTable"
import { InventoryEntryFormDrawer } from "./InventoryEntryFormDrawer"

const LIMIT = 20
// The API's max page size (`limit` maxes out at 100). This is a best-effort
// lookup for resolving medicine_id -> name/barcode: it only covers the first
// 100 active medicines and excludes deleted ones. An entry referencing a
// medicine outside that set (more medicines than this, or one deleted since
// the entry was recorded) falls back to showing its raw medicine_id.
const MEDICINE_LOOKUP_LIMIT = 100
const FALLBACK_ERROR = "Something went wrong. Please try again."

export function InventoryEntriesPageClient() {
  const [entries, setEntries] = useState<InventoryEntry[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [medicines, setMedicines] = useState<Medicine[]>([])
  const [drawerOpen, setDrawerOpen] = useState(false)

  const fetchEntries = useCallback(async (targetPage: number) => {
    setLoading(true)
    setError(null)

    const result = await listInventoryEntries({
      limit: LIMIT,
      offset: (targetPage - 1) * LIMIT,
    })

    setLoading(false)
    if (!result.success || !result.data) {
      setError(result.error ?? FALLBACK_ERROR)
      return
    }

    setEntries(result.data.data)
    setTotal(result.data.total)
  }, [])

  useEffect(() => {
    fetchEntries(page)
  }, [page, fetchEntries])

  useEffect(() => {
    listMedicines({ limit: MEDICINE_LOOKUP_LIMIT, offset: 0 }).then((result) => {
      if (result.success && result.data) setMedicines(result.data.data)
    })
  }, [])

  const medicinesById = useMemo(
    () => new Map(medicines.map((medicine) => [medicine.id, medicine])),
    [medicines],
  )

  return (
    <Stack gap="6">
      <Flex justify="space-between" align="center" wrap="wrap" gap="4">
        <Heading size="lg">Inventory Entries</Heading>
        <Button onClick={() => setDrawerOpen(true)}>Record Adjustment</Button>
      </Flex>

      {error ? (
        <Alert status="error" title="Couldn't load inventory entries">
          {error}
        </Alert>
      ) : loading ? (
        <Center py="12">
          <Spinner />
        </Center>
      ) : (
        <InventoryEntriesTable entries={entries} medicinesById={medicinesById} />
      )}

      {total > 0 && (
        <PaginationRoot
          count={total}
          pageSize={LIMIT}
          page={page}
          onPageChange={(details) => setPage(details.page)}
        >
          <HStack justify="flex-end">
            <PaginationPrevTrigger />
            <PaginationPageText />
            <PaginationNextTrigger />
          </HStack>
        </PaginationRoot>
      )}

      <InventoryEntryFormDrawer
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        onSaved={() => fetchEntries(page)}
      />
    </Stack>
  )
}
