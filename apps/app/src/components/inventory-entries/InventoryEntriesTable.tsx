"use client"

import { Table, Text } from "@chakra-ui/react"
import { EmptyState } from "@inventramed/snippets/empty-state"
import { Tag } from "@inventramed/snippets/tag"
import { HiOutlineClipboardDocumentList } from "react-icons/hi2"
import type { Medicine } from "../../types/medicine"
import type { InventoryEntry } from "../../types/inventory-entry"

const EMPTY_VALUE = "—"

const DIRECTION_LABEL = {
  addition: "Addition",
  subtraction: "Subtraction",
} as const

const DIRECTION_COLOR = {
  addition: "green",
  subtraction: "red",
} as const

interface InventoryEntriesTableProps {
  entries: InventoryEntry[]
  medicinesById: Map<string, Medicine>
}

export function InventoryEntriesTable({
  entries,
  medicinesById,
}: InventoryEntriesTableProps) {
  if (entries.length === 0) {
    return (
      <EmptyState
        icon={<HiOutlineClipboardDocumentList />}
        title="No inventory entries yet"
        description="Record a stock adjustment to see it here."
      />
    )
  }

  return (
    <Table.ScrollArea borderWidth="1px" rounded="md">
      <Table.Root size="sm" stickyHeader>
        <Table.Header>
          <Table.Row>
            <Table.ColumnHeader>Date</Table.ColumnHeader>
            <Table.ColumnHeader>Medicine</Table.ColumnHeader>
            <Table.ColumnHeader>Direction</Table.ColumnHeader>
            <Table.ColumnHeader textAlign="end">Quantity</Table.ColumnHeader>
            <Table.ColumnHeader>Reason</Table.ColumnHeader>
            <Table.ColumnHeader>Notes</Table.ColumnHeader>
            <Table.ColumnHeader>Counted By</Table.ColumnHeader>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {entries.map((entry) => {
            const medicine = medicinesById.get(entry.medicine_id)
            return (
              <Table.Row key={entry.id}>
                <Table.Cell>
                  {new Date(entry.created_at).toLocaleString()}
                </Table.Cell>
                <Table.Cell>
                  {medicine ? (
                    <>
                      {medicine.name}
                      <Text as="span" color="fg.muted">
                        {" "}
                        ({medicine.barcode})
                      </Text>
                    </>
                  ) : (
                    <Text as="span" fontFamily="mono" color="fg.muted">
                      {entry.medicine_id}
                    </Text>
                  )}
                </Table.Cell>
                <Table.Cell>
                  <Tag colorPalette={DIRECTION_COLOR[entry.direction]}>
                    {DIRECTION_LABEL[entry.direction]}
                  </Tag>
                </Table.Cell>
                <Table.Cell textAlign="end">{entry.quantity}</Table.Cell>
                <Table.Cell>{entry.reason}</Table.Cell>
                <Table.Cell>{entry.notes ?? EMPTY_VALUE}</Table.Cell>
                <Table.Cell>{entry.counted_by.name || EMPTY_VALUE}</Table.Cell>
              </Table.Row>
            )
          })}
        </Table.Body>
      </Table.Root>
    </Table.ScrollArea>
  )
}
