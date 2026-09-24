"use client"

import { Button, Input, Stack } from "@chakra-ui/react"
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
import { createMedicine, updateMedicine } from "../../actions/medicines"
import { LOCATIONS, locationLabel } from "../../lib/live"
import type { Medicine } from "../../types/medicine"

const FALLBACK_ERROR = "Something went wrong. Please try again."

// The empty value is "Not placed" and is sent as a null location.
const LOCATION_ITEMS = [
  { value: "", label: "Not placed" },
  ...LOCATIONS.map((location) => ({
    value: String(location),
    label: locationLabel(location),
  })),
]

interface MedicineFormDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  medicine: Medicine | null
  onSaved: () => void
}

export function MedicineFormDrawer({
  open,
  onOpenChange,
  medicine,
  onSaved,
}: MedicineFormDrawerProps) {
  const isEdit = medicine != null

  const [name, setName] = useState("")
  const [barcode, setBarcode] = useState("")
  const [batchNumber, setBatchNumber] = useState("")
  const [expirationDate, setExpirationDate] = useState("")
  const [quantity, setQuantity] = useState("0")
  const [location, setLocation] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return

    setName(medicine?.name ?? "")
    setBarcode(medicine?.barcode ?? "")
    setBatchNumber(medicine?.batch_number ?? "")
    setExpirationDate(medicine?.expiration_date ?? "")
    setQuantity(medicine ? String(medicine.quantity) : "0")
    setLocation(medicine?.location != null ? String(medicine.location) : "")
    setError(null)
  }, [open, medicine])

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitting(true)
    setError(null)

    const batch = batchNumber.trim() || null
    const placedAt = location ? Number(location) : null
    const result = medicine
      ? await updateMedicine(medicine.id, {
          name: name.trim(),
          barcode: barcode.trim(),
          batch_number: batch,
          expiration_date: expirationDate,
          location: placedAt,
        })
      : await createMedicine({
          name: name.trim(),
          barcode: barcode.trim(),
          batch_number: batch,
          expiration_date: expirationDate,
          quantity: Number(quantity),
          location: placedAt,
        })

    setSubmitting(false)
    if (!result.success) {
      setError(result.error ?? FALLBACK_ERROR)
      return
    }

    toaster.create({
      type: "success",
      title: isEdit ? "Medicine updated" : "Medicine created",
    })
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
            <DrawerTitle>{isEdit ? "Edit Medicine" : "New Medicine"}</DrawerTitle>
          </DrawerHeader>
          <DrawerBody>
            <Stack gap="4">
              {error && (
                <Alert status="error" title="Couldn't save medicine">
                  {error}
                </Alert>
              )}
              <Field label="Name" required>
                <Input
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  required
                />
              </Field>
              <Field label="Barcode" required>
                <Input
                  value={barcode}
                  onChange={(event) => setBarcode(event.target.value)}
                  required
                />
              </Field>
              <Field label="Batch Number" optionalText="(optional)">
                <Input
                  value={batchNumber}
                  onChange={(event) => setBatchNumber(event.target.value)}
                />
              </Field>
              <Field label="Expiration Date" required>
                <Input
                  type="date"
                  value={expirationDate}
                  onChange={(event) => setExpirationDate(event.target.value)}
                  required
                />
              </Field>
              <Field label="Location" optionalText="(optional)">
                <NativeSelectRoot>
                  <NativeSelectField
                    items={LOCATION_ITEMS}
                    value={location}
                    onChange={(event) => setLocation(event.target.value)}
                  />
                </NativeSelectRoot>
              </Field>
              {!isEdit && (
                <Field label="Quantity" required>
                  <NumberInputRoot
                    width="full"
                    min={0}
                    value={quantity}
                    onValueChange={(details) => setQuantity(details.value)}
                  >
                    <NumberInputField required />
                  </NumberInputRoot>
                </Field>
              )}
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
            <Button type="submit" loading={submitting}>
              Save
            </Button>
          </DrawerFooter>
        </form>
      </DrawerContent>
    </DrawerRoot>
  )
}
