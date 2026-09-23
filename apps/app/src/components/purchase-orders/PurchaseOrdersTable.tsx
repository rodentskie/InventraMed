"use client"

import { Table, Text } from "@chakra-ui/react"
import { EmptyState } from "@inventramed/snippets/empty-state"
import type { ReactNode } from "react"
import { HiOutlineClipboardDocumentList } from "react-icons/hi2"
import type { PurchaseOrder } from "../../types/purchase-order"
import type { Supplier } from "../../types/supplier"
import { PurchaseOrderStatusTag } from "./PurchaseOrderStatusTag"

const EMPTY_VALUE = "—"

interface PurchaseOrdersTableProps {
  purchaseOrders: PurchaseOrder[]
  suppliersById: Map<string, Supplier>
  renderRowAction: (purchaseOrder: PurchaseOrder) => ReactNode
}

export function PurchaseOrdersTable({
  purchaseOrders,
  suppliersById,
  renderRowAction,
}: PurchaseOrdersTableProps) {
  if (purchaseOrders.length === 0) {
    return (
      <EmptyState
        icon={<HiOutlineClipboardDocumentList />}
        title="No purchase orders yet"
        description="Create a purchase order to start restocking from a supplier."
      />
    )
  }

  return (
    <Table.ScrollArea borderWidth="1px" rounded="md">
      <Table.Root size="sm" stickyHeader>
        <Table.Header>
          <Table.Row>
            <Table.ColumnHeader>Supplier</Table.ColumnHeader>
            <Table.ColumnHeader>Status</Table.ColumnHeader>
            <Table.ColumnHeader>Order Date</Table.ColumnHeader>
            <Table.ColumnHeader>Expected Date</Table.ColumnHeader>
            <Table.ColumnHeader>Created At</Table.ColumnHeader>
            <Table.ColumnHeader textAlign="end">Actions</Table.ColumnHeader>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {purchaseOrders.map((purchaseOrder) => {
            const supplier = suppliersById.get(purchaseOrder.supplier_id)
            return (
              <Table.Row key={purchaseOrder.id}>
                <Table.Cell>
                  {supplier ? (
                    supplier.name
                  ) : (
                    <Text as="span" fontFamily="mono" color="fg.muted">
                      {purchaseOrder.supplier_id}
                    </Text>
                  )}
                </Table.Cell>
                <Table.Cell>
                  <PurchaseOrderStatusTag status={purchaseOrder.status} />
                </Table.Cell>
                <Table.Cell>{purchaseOrder.order_date}</Table.Cell>
                <Table.Cell>
                  {purchaseOrder.expected_date ?? EMPTY_VALUE}
                </Table.Cell>
                <Table.Cell>
                  {new Date(purchaseOrder.created_at).toLocaleString()}
                </Table.Cell>
                <Table.Cell textAlign="end">
                  {renderRowAction(purchaseOrder)}
                </Table.Cell>
              </Table.Row>
            )
          })}
        </Table.Body>
      </Table.Root>
    </Table.ScrollArea>
  )
}
