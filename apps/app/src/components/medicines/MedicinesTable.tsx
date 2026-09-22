"use client"

import { HStack, IconButton, Table } from "@chakra-ui/react"
import { EmptyState } from "@inventramed/snippets/empty-state"
import { Status } from "@inventramed/snippets/status"
import { HiOutlineArchiveBox, HiOutlinePencil, HiOutlineTrash } from "react-icons/hi2"
import { getMedicineStatus } from "../../lib/medicine-status"
import type { Medicine } from "../../types/medicine"

const STATUS_LABEL = {
  green: "Fresh",
  yellow: "Nearing expiration",
  red: "Expired",
} as const

const STATUS_VALUE = {
  green: "success",
  yellow: "warning",
  red: "error",
} as const

const EMPTY_VALUE = "—"

interface MedicinesTableProps {
  medicines: Medicine[]
  onEdit: (medicine: Medicine) => void
  onDelete: (medicine: Medicine) => void
}

export function MedicinesTable({ medicines, onEdit, onDelete }: MedicinesTableProps) {
  if (medicines.length === 0) {
    return (
      <EmptyState
        icon={<HiOutlineArchiveBox />}
        title="No medicines yet"
        description="Register a medicine to start tracking its expiration."
      />
    )
  }

  return (
    <Table.ScrollArea borderWidth="1px" rounded="md">
      <Table.Root size="sm" stickyHeader>
        <Table.Header>
          <Table.Row>
            <Table.ColumnHeader>Name</Table.ColumnHeader>
            <Table.ColumnHeader>Barcode</Table.ColumnHeader>
            <Table.ColumnHeader>Batch Number</Table.ColumnHeader>
            <Table.ColumnHeader>Expiration Date</Table.ColumnHeader>
            <Table.ColumnHeader textAlign="end">Quantity</Table.ColumnHeader>
            <Table.ColumnHeader>Status</Table.ColumnHeader>
            <Table.ColumnHeader textAlign="end">Actions</Table.ColumnHeader>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {medicines.map((medicine) => {
            const status = getMedicineStatus(medicine.expiration_date)
            return (
              <Table.Row key={medicine.id}>
                <Table.Cell>{medicine.name}</Table.Cell>
                <Table.Cell>{medicine.barcode}</Table.Cell>
                <Table.Cell>{medicine.batch_number ?? EMPTY_VALUE}</Table.Cell>
                <Table.Cell>{medicine.expiration_date}</Table.Cell>
                <Table.Cell textAlign="end">{medicine.quantity}</Table.Cell>
                <Table.Cell>
                  <Status value={STATUS_VALUE[status]}>{STATUS_LABEL[status]}</Status>
                </Table.Cell>
                <Table.Cell textAlign="end">
                  <HStack gap="1" justify="flex-end">
                    <IconButton
                      aria-label={`Edit ${medicine.name}`}
                      variant="ghost"
                      size="sm"
                      onClick={() => onEdit(medicine)}
                    >
                      <HiOutlinePencil />
                    </IconButton>
                    <IconButton
                      aria-label={`Delete ${medicine.name}`}
                      variant="ghost"
                      colorPalette="red"
                      size="sm"
                      onClick={() => onDelete(medicine)}
                    >
                      <HiOutlineTrash />
                    </IconButton>
                  </HStack>
                </Table.Cell>
              </Table.Row>
            )
          })}
        </Table.Body>
      </Table.Root>
    </Table.ScrollArea>
  )
}
