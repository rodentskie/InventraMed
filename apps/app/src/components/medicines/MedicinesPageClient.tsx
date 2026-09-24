"use client"

import { Button, Center, Flex, HStack, Heading, Spinner, Stack } from "@chakra-ui/react"
import { Alert } from "@inventramed/snippets/alert"
import {
  PaginationNextTrigger,
  PaginationPageText,
  PaginationPrevTrigger,
  PaginationRoot,
} from "@inventramed/snippets/pagination"
import { useCallback, useEffect, useState } from "react"
import { listMedicines } from "../../actions/medicines"
import type { Medicine } from "../../types/medicine"
import { useWarningThreshold } from "../settings/useWarningThreshold"
import { DeleteMedicineDialog } from "./DeleteMedicineDialog"
import { MedicineFormDrawer } from "./MedicineFormDrawer"
import { MedicinesTable } from "./MedicinesTable"

const LIMIT = 20
const FALLBACK_ERROR = "Something went wrong. Please try again."

export function MedicinesPageClient() {
  const [medicines, setMedicines] = useState<Medicine[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingMedicine, setEditingMedicine] = useState<Medicine | null>(null)
  const [deletingMedicine, setDeletingMedicine] = useState<Medicine | null>(null)
  const thresholdDays = useWarningThreshold()

  const fetchMedicines = useCallback(async (targetPage: number) => {
    setLoading(true)
    setError(null)

    const result = await listMedicines({
      limit: LIMIT,
      offset: (targetPage - 1) * LIMIT,
    })

    setLoading(false)
    if (!result.success || !result.data) {
      setError(result.error ?? FALLBACK_ERROR)
      return
    }

    setMedicines(result.data.data)
    setTotal(result.data.total)
  }, [])

  useEffect(() => {
    fetchMedicines(page)
  }, [page, fetchMedicines])

  const openCreateDrawer = () => {
    setEditingMedicine(null)
    setDrawerOpen(true)
  }

  const openEditDrawer = (medicine: Medicine) => {
    setEditingMedicine(medicine)
    setDrawerOpen(true)
  }

  return (
    <Stack gap="6">
      <Flex justify="space-between" align="center" wrap="wrap" gap="4">
        <Heading size="lg">Medicines</Heading>
        <Button onClick={openCreateDrawer}>New Medicine</Button>
      </Flex>

      {error ? (
        <Alert status="error" title="Couldn't load medicines">
          {error}
        </Alert>
      ) : loading ? (
        <Center py="12">
          <Spinner />
        </Center>
      ) : (
        <MedicinesTable
          medicines={medicines}
          thresholdDays={thresholdDays}
          onEdit={openEditDrawer}
          onDelete={setDeletingMedicine}
        />
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

      <MedicineFormDrawer
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        medicine={editingMedicine}
        onSaved={() => fetchMedicines(page)}
      />
      <DeleteMedicineDialog
        medicine={deletingMedicine}
        onOpenChange={(open) => !open && setDeletingMedicine(null)}
        onDeleted={() => fetchMedicines(page)}
      />
    </Stack>
  )
}
