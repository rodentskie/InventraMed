"use client"

import { Button, Center, Flex, Heading, Spinner, Stack, Table, Text } from "@chakra-ui/react"
import { Alert } from "@inventramed/snippets/alert"
import { DataListItem, DataListRoot } from "@inventramed/snippets/data-list"
import { EmptyState } from "@inventramed/snippets/empty-state"
import NextLink from "next/link"
import { useCallback, useEffect, useState } from "react"
import { HiOutlineInbox } from "react-icons/hi2"
import { getPurchaseOrder } from "../../actions/purchase-orders"
import type {
  PurchaseOrderDetail as PurchaseOrderDetailData,
  PurchaseOrderReceiptItem,
} from "../../types/purchase-order"
import { isPurchaseOrderReceivable, PurchaseOrderStatusTag } from "./PurchaseOrderStatusTag"
import { useMedicineLookup } from "./useMedicineLookup"
import { useSupplierLookup } from "./useSupplierLookup"

const EMPTY_VALUE = "—"
const FALLBACK_ERROR = "Something went wrong. Please try again."

interface PurchaseOrderDetailProps {
  id: string
}

export function PurchaseOrderDetail({ id }: PurchaseOrderDetailProps) {
  const [purchaseOrder, setPurchaseOrder] = useState<PurchaseOrderDetailData | null>(
    null,
  )
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const suppliersById = useSupplierLookup()
  const medicinesById = useMedicineLookup()

  const fetchPurchaseOrder = useCallback(async () => {
    setLoading(true)
    setError(null)

    const result = await getPurchaseOrder(id)
    setLoading(false)
    if (!result.success || !result.data) {
      setError(result.error ?? FALLBACK_ERROR)
      return
    }

    setPurchaseOrder(result.data)
  }, [id])

  useEffect(() => {
    fetchPurchaseOrder()
  }, [fetchPurchaseOrder])

  const medicineLabel = useCallback(
    (medicineId: string) => medicinesById.get(medicineId)?.name ?? medicineId,
    [medicinesById],
  )

  const receiptItemMedicineLabel = useCallback(
    (item: PurchaseOrderReceiptItem, items: PurchaseOrderDetailData["items"]) => {
      const orderItem = items.find((i) => i.id === item.purchase_order_item_id)
      return orderItem ? medicineLabel(orderItem.medicine_id) : item.purchase_order_item_id
    },
    [medicineLabel],
  )

  if (loading) {
    return (
      <Center py="12">
        <Spinner />
      </Center>
    )
  }

  if (error || !purchaseOrder) {
    return (
      <Stack gap="4">
        <Alert status="error" title="Couldn't load purchase order">
          {error ?? FALLBACK_ERROR}
        </Alert>
        <Button asChild variant="outline" alignSelf="flex-start">
          <NextLink href="/purchase-orders">Back to Purchase Orders</NextLink>
        </Button>
      </Stack>
    )
  }

  const supplier = suppliersById.get(purchaseOrder.supplier_id)

  return (
    <Stack gap="6">
      <Flex justify="space-between" align="center" wrap="wrap" gap="4">
        <Stack gap="1">
          <Button asChild variant="plain" size="sm" px="0" justifyContent="flex-start">
            <NextLink href="/purchase-orders">&larr; Back to Purchase Orders</NextLink>
          </Button>
          <Heading size="lg">
            Purchase Order{" "}
            <Text as="span" fontFamily="mono" fontSize="lg" color="fg.muted">
              #{purchaseOrder.id.slice(0, 8)}
            </Text>
          </Heading>
        </Stack>
        {isPurchaseOrderReceivable(purchaseOrder.status) && (
          <Button asChild>
            <NextLink href={`/purchase-orders/receive?id=${purchaseOrder.id}`}>
              Receive
            </NextLink>
          </Button>
        )}
      </Flex>

      <DataListRoot orientation="horizontal" gap="3">
        <DataListItem
          label="Supplier"
          value={supplier ? supplier.name : purchaseOrder.supplier_id}
        />
        <DataListItem
          label="Status"
          value={<PurchaseOrderStatusTag status={purchaseOrder.status} />}
        />
        <DataListItem label="Order Date" value={purchaseOrder.order_date} />
        <DataListItem
          label="Expected Date"
          value={purchaseOrder.expected_date ?? EMPTY_VALUE}
        />
        <DataListItem label="Notes" value={purchaseOrder.notes ?? EMPTY_VALUE} />
        <DataListItem
          label="Created At"
          value={new Date(purchaseOrder.created_at).toLocaleString()}
        />
      </DataListRoot>

      <Stack gap="3">
        <Heading size="md">Items</Heading>
        <Table.ScrollArea borderWidth="1px" rounded="md">
          <Table.Root size="sm">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>Medicine</Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  Quantity Ordered
                </Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {purchaseOrder.items.map((item) => (
                <Table.Row key={item.id}>
                  <Table.Cell>{medicineLabel(item.medicine_id)}</Table.Cell>
                  <Table.Cell textAlign="end">{item.quantity_ordered}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </Table.ScrollArea>
      </Stack>

      <Stack gap="3">
        <Heading size="md">Receipts</Heading>
        {purchaseOrder.receipts.length === 0 ? (
          <EmptyState icon={<HiOutlineInbox />} title="Not received yet" />
        ) : (
          purchaseOrder.receipts.map((receipt) => (
            <Stack key={receipt.id} gap="3" borderWidth="1px" rounded="md" p="4">
              <DataListRoot orientation="horizontal" gap="2">
                <DataListItem
                  label="Received At"
                  value={new Date(receipt.received_at).toLocaleString()}
                />
                <DataListItem label="Notes" value={receipt.notes ?? EMPTY_VALUE} />
              </DataListRoot>
              <Table.ScrollArea borderWidth="1px" rounded="md">
                <Table.Root size="sm">
                  <Table.Header>
                    <Table.Row>
                      <Table.ColumnHeader>Medicine</Table.ColumnHeader>
                      <Table.ColumnHeader textAlign="end">
                        Received
                      </Table.ColumnHeader>
                      <Table.ColumnHeader textAlign="end">
                        Damaged
                      </Table.ColumnHeader>
                      <Table.ColumnHeader textAlign="end">
                        Returned
                      </Table.ColumnHeader>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {receipt.items.map((item) => (
                      <Table.Row key={item.id}>
                        <Table.Cell>
                          {receiptItemMedicineLabel(item, purchaseOrder.items)}
                        </Table.Cell>
                        <Table.Cell textAlign="end">
                          {item.quantity_received}
                        </Table.Cell>
                        <Table.Cell textAlign="end">
                          {item.quantity_damaged}
                        </Table.Cell>
                        <Table.Cell textAlign="end">
                          {item.quantity_returned}
                        </Table.Cell>
                      </Table.Row>
                    ))}
                  </Table.Body>
                </Table.Root>
              </Table.ScrollArea>
            </Stack>
          ))
        )}
      </Stack>
    </Stack>
  )
}
