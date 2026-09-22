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
import { toaster } from "@inventramed/snippets/toaster"
import { useEffect, useState } from "react"
import { createSupplier, updateSupplier } from "../../actions/suppliers"
import type { Supplier } from "../../types/supplier"

const FALLBACK_ERROR = "Something went wrong. Please try again."

interface SupplierFormDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  supplier: Supplier | null
  onSaved: () => void
}

export function SupplierFormDrawer({
  open,
  onOpenChange,
  supplier,
  onSaved,
}: SupplierFormDrawerProps) {
  const isEdit = supplier != null

  const [name, setName] = useState("")
  const [contactName, setContactName] = useState("")
  const [email, setEmail] = useState("")
  const [phone, setPhone] = useState("")
  const [address, setAddress] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return

    setName(supplier?.name ?? "")
    setContactName(supplier?.contact_name ?? "")
    setEmail(supplier?.email ?? "")
    setPhone(supplier?.phone ?? "")
    setAddress(supplier?.address ?? "")
    setError(null)
  }, [open, supplier])

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitting(true)
    setError(null)

    const input = {
      name: name.trim(),
      contact_name: contactName.trim() || null,
      email: email.trim() || null,
      phone: phone.trim() || null,
      address: address.trim() || null,
    }
    const result = supplier
      ? await updateSupplier(supplier.id, input)
      : await createSupplier(input)

    setSubmitting(false)
    if (!result.success) {
      setError(result.error ?? FALLBACK_ERROR)
      return
    }

    toaster.create({
      type: "success",
      title: isEdit ? "Supplier updated" : "Supplier created",
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
            <DrawerTitle>{isEdit ? "Edit Supplier" : "New Supplier"}</DrawerTitle>
          </DrawerHeader>
          <DrawerBody>
            <Stack gap="4">
              {error && (
                <Alert status="error" title="Couldn't save supplier">
                  {error}
                </Alert>
              )}
              <Field label="Name" required>
                <Input
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  maxLength={255}
                  required
                />
              </Field>
              <Field label="Contact Name" optionalText="(optional)">
                <Input
                  value={contactName}
                  onChange={(event) => setContactName(event.target.value)}
                  maxLength={255}
                />
              </Field>
              <Field label="Email" optionalText="(optional)">
                <Input
                  type="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  maxLength={255}
                />
              </Field>
              <Field label="Phone" optionalText="(optional)">
                <Input
                  value={phone}
                  onChange={(event) => setPhone(event.target.value)}
                  maxLength={32}
                />
              </Field>
              <Field label="Address" optionalText="(optional)">
                <Textarea
                  value={address}
                  onChange={(event) => setAddress(event.target.value)}
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
            <Button type="submit" loading={submitting}>
              Save
            </Button>
          </DrawerFooter>
        </form>
      </DrawerContent>
    </DrawerRoot>
  )
}
