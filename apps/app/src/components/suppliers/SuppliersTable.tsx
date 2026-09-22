"use client"

import { HStack, IconButton, Table } from "@chakra-ui/react"
import { EmptyState } from "@inventramed/snippets/empty-state"
import { HiOutlinePencil, HiOutlineTrash, HiOutlineTruck } from "react-icons/hi2"
import type { Supplier } from "../../types/supplier"

const EMPTY_VALUE = "—"

interface SuppliersTableProps {
  suppliers: Supplier[]
  onEdit: (supplier: Supplier) => void
  onDelete: (supplier: Supplier) => void
}

export function SuppliersTable({ suppliers, onEdit, onDelete }: SuppliersTableProps) {
  if (suppliers.length === 0) {
    return (
      <EmptyState
        icon={<HiOutlineTruck />}
        title="No suppliers yet"
        description="Register a supplier to start ordering from them."
      />
    )
  }

  return (
    <Table.ScrollArea borderWidth="1px" rounded="md">
      <Table.Root size="sm" stickyHeader>
        <Table.Header>
          <Table.Row>
            <Table.ColumnHeader>Name</Table.ColumnHeader>
            <Table.ColumnHeader>Contact Name</Table.ColumnHeader>
            <Table.ColumnHeader>Email</Table.ColumnHeader>
            <Table.ColumnHeader>Phone</Table.ColumnHeader>
            <Table.ColumnHeader>Address</Table.ColumnHeader>
            <Table.ColumnHeader textAlign="end">Actions</Table.ColumnHeader>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {suppliers.map((supplier) => (
            <Table.Row key={supplier.id}>
              <Table.Cell>{supplier.name}</Table.Cell>
              <Table.Cell>{supplier.contact_name ?? EMPTY_VALUE}</Table.Cell>
              <Table.Cell>{supplier.email ?? EMPTY_VALUE}</Table.Cell>
              <Table.Cell>{supplier.phone ?? EMPTY_VALUE}</Table.Cell>
              <Table.Cell>{supplier.address ?? EMPTY_VALUE}</Table.Cell>
              <Table.Cell textAlign="end">
                <HStack gap="1" justify="flex-end">
                  <IconButton
                    aria-label={`Edit ${supplier.name}`}
                    variant="ghost"
                    size="sm"
                    onClick={() => onEdit(supplier)}
                  >
                    <HiOutlinePencil />
                  </IconButton>
                  <IconButton
                    aria-label={`Delete ${supplier.name}`}
                    variant="ghost"
                    colorPalette="red"
                    size="sm"
                    onClick={() => onDelete(supplier)}
                  >
                    <HiOutlineTrash />
                  </IconButton>
                </HStack>
              </Table.Cell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table.Root>
    </Table.ScrollArea>
  )
}
