"use client"

import { Button, Input, Stack, Textarea } from "@chakra-ui/react"
import { Alert } from "@inventramed/snippets/alert"
import {
  DrawerBody,
  DrawerCloseTrigger,
  DrawerContent,
  DrawerFooter,
  DrawerHeader,
  DrawerRoot,
  DrawerTitle,
} from "@inventramed/snippets/drawer"
import { Field } from "@inventramed/snippets/field"
import {
  NativeSelectField,
  NativeSelectRoot,
} from "@inventramed/snippets/native-select"
import {
  NumberInputField,
  NumberInputRoot,
} from "@inventramed/snippets/number-input"
import { toaster } from "@inventramed/snippets/toaster"
import { useEffect, useState } from "react"
import { createInventoryEntry } from "../../actions/inventory-entries"
import type { InventoryEntryDirection } from "../../types/inventory-entry"
import { MedicineCombobox } from "./MedicineCombobox"

const FALLBACK_ERROR = "Something went wrong. Please try again."

interface InventoryEntryFormDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}

export function InventoryEntryFormDrawer({
  open,
  onOpenChange,
  onSaved,
}: InventoryEntryFormDrawerProps) {
  const [medicineId, setMedicineId] = useState("")
  const [direction, setDirection] = useState<InventoryEntryDirection>("addition")
  const [quantity, setQuantity] = useState("1")
  const [reason, setReason] = useState("")
  const [notes, setNotes] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return

    setMedicineId("")
    setDirection("addition")
    setQuantity("1")
    setReason("")
    setNotes("")
    setError(null)
  }, [open])

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitting(true)
    setError(null)

    const result = await createInventoryEntry({
      medicine_id: medicineId,
      direction,
      quantity: Number(quantity),
      reason: reason.trim(),
      notes: notes.trim() || null,
    })

    setSubmitting(false)
    if (!result.success) {
      setError(result.error ?? FALLBACK_ERROR)
      return
    }

    toaster.create({ type: "success", title: "Adjustment recorded" })
    onOpenChange(false)
    onSaved()
  }

  return (
    <DrawerRoot
      open={open}
      placement="end"
      size={{ base: "full", md: "sm" }}
      onOpenChange={(details) => onOpenChange(details.open)}
    >
      <DrawerContent>
        <form onSubmit={handleSubmit}>
          <DrawerCloseTrigger />
          <DrawerHeader>
            <DrawerTitle>Record Stock Adjustment</DrawerTitle>
          </DrawerHeader>
          <DrawerBody>
            <Stack gap="4">
              {error && (
                <Alert status="error" title="Couldn't record adjustment">
                  {error}
                </Alert>
              )}
              <Field label="Medicine" required>
                <MedicineCombobox value={medicineId} onValueChange={setMedicineId} />
              </Field>
              <Field label="Direction" required>
                <NativeSelectRoot>
                  <NativeSelectField
                    value={direction}
                    onChange={(event) =>
                      setDirection(event.target.value as InventoryEntryDirection)
                    }
                    items={[
                      { value: "addition", label: "Addition" },
                      { value: "subtraction", label: "Subtraction" },
                    ]}
                  />
                </NativeSelectRoot>
              </Field>
              <Field label="Quantity" required>
                <NumberInputRoot
                  width="full"
                  min={1}
                  value={quantity}
                  onValueChange={(details) => setQuantity(details.value)}
                >
                  <NumberInputField required />
                </NumberInputRoot>
              </Field>
              <Field label="Reason" required>
                <Input
                  value={reason}
                  onChange={(event) => setReason(event.target.value)}
                  placeholder="e.g. count_adjustment, returned, damaged, lost, expired_removal"
                  maxLength={100}
                  required
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
          </DrawerBody>
          <DrawerFooter>
            <Button
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button type="submit" loading={submitting} disabled={!medicineId}>
              Save
            </Button>
          </DrawerFooter>
        </form>
      </DrawerContent>
    </DrawerRoot>
  )
}
