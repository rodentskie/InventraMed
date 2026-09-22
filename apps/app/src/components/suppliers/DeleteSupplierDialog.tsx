"use client"

import { Button, Input, Stack, Text } from "@chakra-ui/react"
import { deleteSupplier } from "../../actions/suppliers"
import {
  DialogBody,
  DialogCloseTrigger,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogRoot,
  DialogTitle,
} from "@inventramed/snippets/dialog"
import { Field } from "@inventramed/snippets/field"
import { toaster } from "@inventramed/snippets/toaster"
import { useEffect, useState } from "react"
import type { Supplier } from "../../types/supplier"

const FALLBACK_ERROR = "Something went wrong. Please try again."
const CONFIRM_WORD = "delete"

interface DeleteSupplierDialogProps {
  supplier: Supplier | null
  onOpenChange: (open: boolean) => void
  onDeleted: () => void
}

export function DeleteSupplierDialog({
  supplier,
  onOpenChange,
  onDeleted,
}: DeleteSupplierDialogProps) {
  const [confirmText, setConfirmText] = useState("")
  const [deleting, setDeleting] = useState(false)
  const open = supplier != null

  useEffect(() => {
    if (!open) return
    setConfirmText("")
  }, [open])

  const confirmed = confirmText.trim().toLowerCase() === CONFIRM_WORD

  const handleConfirm = async () => {
    if (!supplier) return
    setDeleting(true)

    const result = await deleteSupplier(supplier.id)
    setDeleting(false)
    onOpenChange(false)

    toaster.create(
      result.success
        ? { type: "success", title: "Supplier deleted" }
        : {
            type: "error",
            title: "Couldn't delete supplier",
            description: result.error ?? FALLBACK_ERROR,
          },
    )
    onDeleted()
  }

  return (
    <DialogRoot
      role="alertdialog"
      open={open}
      onOpenChange={(details) => !details.open && onOpenChange(false)}
    >
      <DialogContent>
        <DialogCloseTrigger />
        <DialogHeader>
          <DialogTitle>Delete supplier</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <Stack gap="4">
            <Text>
              This will delete <strong>{supplier?.name}</strong>. This can't
              be undone from here. Type <strong>{CONFIRM_WORD}</strong> below
              to confirm.
            </Text>
            <Field label={`Type "${CONFIRM_WORD}" to confirm`}>
              <Input
                value={confirmText}
                onChange={(event) => setConfirmText(event.target.value)}
                autoComplete="off"
              />
            </Field>
          </Stack>
        </DialogBody>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={deleting}
          >
            Cancel
          </Button>
          <Button
            colorPalette="red"
            disabled={!confirmed}
            loading={deleting}
            onClick={handleConfirm}
          >
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </DialogRoot>
  )
}
