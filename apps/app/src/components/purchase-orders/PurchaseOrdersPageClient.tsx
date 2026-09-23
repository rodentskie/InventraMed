"use client"

import { Button, Center, Flex, HStack, Heading, IconButton, Spinner, Stack } from "@chakra-ui/react"
import { Alert } from "@inventramed/snippets/alert"
import {
  PaginationNextTrigger,
  PaginationPageText,
  PaginationPrevTrigger,
  PaginationRoot,
} from "@inventramed/snippets/pagination"
import NextLink from "next/link"
import { useCallback, useEffect, useState } from "react"
import { HiOutlineEye } from "react-icons/hi2"
import { listPurchaseOrders } from "../../actions/purchase-orders"
import type { PurchaseOrder } from "../../types/purchase-order"
import { PurchaseOrdersTable } from "./PurchaseOrdersTable"
import { useSupplierLookup } from "./useSupplierLookup"

const LIMIT = 20
const FALLBACK_ERROR = "Something went wrong. Please try again."

export function PurchaseOrdersPageClient() {
  const [purchaseOrders, setPurchaseOrders] = useState<PurchaseOrder[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const suppliersById = useSupplierLookup()

  const fetchPurchaseOrders = useCallback(async (targetPage: number) => {
    setLoading(true)
    setError(null)

    const result = await listPurchaseOrders({
      limit: LIMIT,
      offset: (targetPage - 1) * LIMIT,
    })

    setLoading(false)
    if (!result.success || !result.data) {
      setError(result.error ?? FALLBACK_ERROR)
      return
    }

    setPurchaseOrders(result.data.data)
    setTotal(result.data.total)
  }, [])

  useEffect(() => {
    fetchPurchaseOrders(page)
  }, [page, fetchPurchaseOrders])

  return (
    <Stack gap="6">
      <Flex justify="space-between" align="center" wrap="wrap" gap="4">
        <Heading size="lg">Purchase Orders</Heading>
        <Button asChild>
          <NextLink href="/purchase-orders/new">New Purchase Order</NextLink>
        </Button>
      </Flex>

      {error ? (
        <Alert status="error" title="Couldn't load purchase orders">
          {error}
        </Alert>
      ) : loading ? (
        <Center py="12">
          <Spinner />
        </Center>
      ) : (
        <PurchaseOrdersTable
          purchaseOrders={purchaseOrders}
          suppliersById={suppliersById}
          renderRowAction={(purchaseOrder) => (
            <IconButton
              asChild
              aria-label={`View purchase order ${purchaseOrder.id}`}
              variant="ghost"
              size="sm"
            >
              <NextLink href={`/purchase-orders/${purchaseOrder.id}`}>
                <HiOutlineEye />
              </NextLink>
            </IconButton>
          )}
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
    </Stack>
  )
}
