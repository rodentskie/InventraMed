"use client"

import {
  Box,
  Button,
  Flex,
  Heading,
  HStack,
  IconButton,
  Input,
  Stack,
  Text,
  Textarea,
} from "@chakra-ui/react"
import { Alert } from "@inventramed/snippets/alert"
import { Field } from "@inventramed/snippets/field"
import {
  NumberInputField,
  NumberInputRoot,
} from "@inventramed/snippets/number-input"
import { toaster } from "@inventramed/snippets/toaster"
import NextLink from "next/link"
import { useRouter } from "next/navigation"
import { useState } from "react"
import { HiOutlinePlus, HiOutlineTrash } from "react-icons/hi2"
import { createPurchaseOrder } from "../../actions/purchase-orders"
import { MedicineCombobox } from "../inventory-entries/MedicineCombobox"
import { SupplierCombobox } from "./SupplierCombobox"

const FALLBACK_ERROR = "Something went wrong. Please try again."
const MAX_ITEMS = 100

interface ItemRow {
  key: string
  medicineId: string
  quantityOrdered: string
}

function newItemRow(): ItemRow {
  return { key: crypto.randomUUID(), medicineId: "", quantityOrdered: "1" }
}

export function PurchaseOrderFormPage() {
  const router = useRouter()

  const [supplierId, setSupplierId] = useState("")
  const [orderDate, setOrderDate] = useState("")
  const [expectedDate, setExpectedDate] = useState("")
  const [notes, setNotes] = useState("")
  const [items, setItems] = useState<ItemRow[]>([newItemRow()])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const updateItem = (key: string, patch: Partial<ItemRow>) => {
    setItems((current) =>
      current.map((item) => (item.key === key ? { ...item, ...patch } : item)),
    )
  }

  const handleMedicineChange = (key: string, medicineId: string) => {
    const isDuplicate =
      medicineId !== "" &&
      items.some((item) => item.key !== key && item.medicineId === medicineId)
    if (isDuplicate) {
      toaster.create({
        type: "warning",
        title: "Medicine already added",
        description: "Each medicine can only appear once on a purchase order.",
      })
      return
    }
    updateItem(key, { medicineId })
  }

  const addItem = () => {
    if (items.length >= MAX_ITEMS) return
    setItems((current) => [...current, newItemRow()])
  }

  const removeItem = (key: string) => {
    setItems((current) =>
      current.length > 1 ? current.filter((item) => item.key !== key) : current,
    )
  }

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitting(true)
    setError(null)

    const result = await createPurchaseOrder({
      supplier_id: supplierId,
      order_date: orderDate,
      expected_date: expectedDate || null,
      notes: notes.trim() || null,
      items: items.map((item) => ({
        medicine_id: item.medicineId,
        quantity_ordered: Number(item.quantityOrdered),
      })),
    })

    setSubmitting(false)
    if (!result.success || !result.data) {
      setError(result.error ?? FALLBACK_ERROR)
      return
    }

    toaster.create({ type: "success", title: "Purchase order created" })
    router.push(`/purchase-orders/${result.data.id}`)
  }

  return (
    <Stack gap="6" maxW="3xl">
      <Flex justify="space-between" align="center" wrap="wrap" gap="4">
        <Heading size="lg">New Purchase Order</Heading>
        <Button asChild variant="outline">
          <NextLink href="/purchase-orders">Cancel</NextLink>
        </Button>
      </Flex>

      <form onSubmit={handleSubmit}>
        <Stack gap="6">
          {error && (
            <Alert status="error" title="Couldn't create purchase order">
              {error}
            </Alert>
          )}

          <Stack gap="4">
            <Field label="Supplier" required>
              <SupplierCombobox value={supplierId} onValueChange={setSupplierId} />
            </Field>
            <Field label="Order Date" required>
              <Input
                type="date"
                value={orderDate}
                onChange={(event) => setOrderDate(event.target.value)}
                required
              />
            </Field>
            <Field label="Expected Date" optionalText="(optional)">
              <Input
                type="date"
                min={orderDate || undefined}
                value={expectedDate}
                onChange={(event) => setExpectedDate(event.target.value)}
              />
            </Field>
            <Field label="Notes" optionalText="(optional)">
              <Textarea
                value={notes}
                onChange={(event) => setNotes(event.target.value)}
                maxLength={500}
              />
            </Field>
          </Stack>

          <Stack gap="3">
            <Text fontWeight="medium">Items</Text>
            <Stack gap="3">
              {items.map((item, index) => (
                <Box key={item.key} borderWidth="1px" rounded="md" p="4">
                  <HStack align="flex-start" gap="4">
                    <Box flex="1">
                      <Field label={`Medicine ${index + 1}`} required>
                        <MedicineCombobox
                          value={item.medicineId}
                          onValueChange={(medicineId) =>
                            handleMedicineChange(item.key, medicineId)
                          }
                        />
                      </Field>
                    </Box>
                    <Box width="32">
                      <Field label="Quantity" required>
                        <NumberInputRoot
                          width="full"
                          min={1}
                          value={item.quantityOrdered}
                          onValueChange={(details) =>
                            updateItem(item.key, { quantityOrdered: details.value })
                          }
                        >
                          <NumberInputField required />
                        </NumberInputRoot>
                      </Field>
                    </Box>
                    <IconButton
                      aria-label={`Remove item ${index + 1}`}
                      variant="ghost"
                      colorPalette="red"
                      mt="8"
                      disabled={items.length === 1}
                      onClick={() => removeItem(item.key)}
                    >
                      <HiOutlineTrash />
                    </IconButton>
                  </HStack>
                </Box>
              ))}
            </Stack>
            <Button
              variant="outline"
              alignSelf="flex-start"
              disabled={items.length >= MAX_ITEMS}
              onClick={addItem}
            >
              <HiOutlinePlus /> Add Item
            </Button>
          </Stack>

          <HStack justify="flex-end">
            <Button type="submit" loading={submitting}>
              Create Purchase Order
            </Button>
          </HStack>
        </Stack>
      </form>
    </Stack>
  )
}
