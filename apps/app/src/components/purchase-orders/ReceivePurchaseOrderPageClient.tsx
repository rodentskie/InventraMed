"use client"

import { Button, Center, Flex, HStack, Heading, Spinner, Stack } from "@chakra-ui/react"
import { Alert } from "@inventramed/snippets/alert"
import {
  PaginationNextTrigger,
  PaginationPageText,
  PaginationPrevTrigger,
  PaginationRoot,
} from "@inventramed/snippets/pagination"
import { useRouter, useSearchParams } from "next/navigation"
import { useCallback, useEffect, useState } from "react"
import { listPurchaseOrders } from "../../actions/purchase-orders"
import type { PurchaseOrder } from "../../types/purchase-order"
import { isPurchaseOrderReceivable } from "./PurchaseOrderStatusTag"
import { PurchaseOrdersTable } from "./PurchaseOrdersTable"
import { ReceivePurchaseOrderDialog } from "./ReceivePurchaseOrderDialog"
import { useSupplierLookup } from "./useSupplierLookup"

const LIMIT = 20
const FALLBACK_ERROR = "Something went wrong. Please try again."

export function ReceivePurchaseOrderPageClient() {
  const router = useRouter()
  const searchParams = useSearchParams()

  const [purchaseOrders, setPurchaseOrders] = useState<PurchaseOrder[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [receivingId, setReceivingId] = useState<string | null>(
    searchParams.get("id"),
  )

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

  const closeDialog = () => {
    setReceivingId(null)
    if (searchParams.get("id")) router.replace("/purchase-orders/receive")
  }

  return (
    <Stack gap="6">
      <Flex justify="space-between" align="center" wrap="wrap" gap="4">
        <Heading size="lg">Receive Purchase Order</Heading>
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
            <Button
              size="sm"
              variant="outline"
              disabled={!isPurchaseOrderReceivable(purchaseOrder.status)}
              onClick={() => setReceivingId(purchaseOrder.id)}
            >
              Receive
            </Button>
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

      <ReceivePurchaseOrderDialog
        purchaseOrderId={receivingId}
        suppliersById={suppliersById}
        onOpenChange={(open) => !open && closeDialog()}
        onReceived={() => fetchPurchaseOrders(page)}
      />
    </Stack>
  )
}
