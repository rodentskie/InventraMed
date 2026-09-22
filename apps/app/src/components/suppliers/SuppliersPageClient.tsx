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
import { listSuppliers } from "../../actions/suppliers"
import type { Supplier } from "../../types/supplier"
import { DeleteSupplierDialog } from "./DeleteSupplierDialog"
import { SupplierFormDrawer } from "./SupplierFormDrawer"
import { SuppliersTable } from "./SuppliersTable"

const LIMIT = 20
const FALLBACK_ERROR = "Something went wrong. Please try again."

export function SuppliersPageClient() {
  const [suppliers, setSuppliers] = useState<Supplier[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingSupplier, setEditingSupplier] = useState<Supplier | null>(null)
  const [deletingSupplier, setDeletingSupplier] = useState<Supplier | null>(null)

  const fetchSuppliers = useCallback(async (targetPage: number) => {
    setLoading(true)
    setError(null)

    const result = await listSuppliers({
      limit: LIMIT,
      offset: (targetPage - 1) * LIMIT,
    })

    setLoading(false)
    if (!result.success || !result.data) {
      setError(result.error ?? FALLBACK_ERROR)
      return
    }

    setSuppliers(result.data.data)
    setTotal(result.data.total)
  }, [])

  useEffect(() => {
    fetchSuppliers(page)
  }, [page, fetchSuppliers])

  const openCreateDrawer = () => {
    setEditingSupplier(null)
    setDrawerOpen(true)
  }

  const openEditDrawer = (supplier: Supplier) => {
    setEditingSupplier(supplier)
    setDrawerOpen(true)
  }

  return (
    <Stack gap="6">
      <Flex justify="space-between" align="center" wrap="wrap" gap="4">
        <Heading size="lg">Suppliers</Heading>
        <Button onClick={openCreateDrawer}>New Supplier</Button>
      </Flex>

      {error ? (
        <Alert status="error" title="Couldn't load suppliers">
          {error}
        </Alert>
      ) : loading ? (
        <Center py="12">
          <Spinner />
        </Center>
      ) : (
        <SuppliersTable
          suppliers={suppliers}
          onEdit={openEditDrawer}
          onDelete={setDeletingSupplier}
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

      <SupplierFormDrawer
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        supplier={editingSupplier}
        onSaved={() => fetchSuppliers(page)}
      />
      <DeleteSupplierDialog
        supplier={deletingSupplier}
        onOpenChange={(open) => !open && setDeletingSupplier(null)}
        onDeleted={() => fetchSuppliers(page)}
      />
    </Stack>
  )
}
