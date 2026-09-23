"use client"

import { Button, Center, Spinner, Stack, Text, Textarea } from "@chakra-ui/react"
import { getPurchaseOrder, receivePurchaseOrder } from "../../actions/purchase-orders"
import {
  DialogBody,
  DialogCloseTrigger,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogRoot,
  DialogTitle,
} from "@inventramed/snippets/dialog"
import { Alert } from "@inventramed/snippets/alert"
import { Field } from "@inventramed/snippets/field"
import { toaster } from "@inventramed/snippets/toaster"
import { useEffect, useState } from "react"
import type { PurchaseOrderDetail } from "../../types/purchase-order"
import type { Supplier } from "../../types/supplier"

const FALLBACK_ERROR = "Something went wrong. Please try again."

interface ReceivePurchaseOrderDialogProps {
  purchaseOrderId: string | null
  suppliersById: Map<string, Supplier>
  onOpenChange: (open: boolean) => void
  onReceived: () => void
}

export function ReceivePurchaseOrderDialog({
  purchaseOrderId,
  suppliersById,
  onOpenChange,
  onReceived,
}: ReceivePurchaseOrderDialogProps) {
  const open = purchaseOrderId != null

  const [detail, setDetail] = useState<PurchaseOrderDetail | null>(null)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [notes, setNotes] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!purchaseOrderId) {
      setDetail(null)
      return
    }

    setNotes("")
    setError(null)
    setLoadError(null)
    setLoadingDetail(true)
    getPurchaseOrder(purchaseOrderId).then((result) => {
      setLoadingDetail(false)
      if (!result.success || !result.data) {
        setLoadError(result.error ?? FALLBACK_ERROR)
        return
      }
      setDetail(result.data)
    })
  }, [purchaseOrderId])

  const handleConfirm = async () => {
    if (!purchaseOrderId) return
    setSubmitting(true)
    setError(null)

    const result = await receivePurchaseOrder(purchaseOrderId, {
      notes: notes.trim() || null,
    })

    setSubmitting(false)

    if (!result.success) {
      // 400 keeps the dialog open for correction; 404/409 close it and toast
      // the server's message instead of retrying.
      if (result.error && result.error.toLowerCase().includes("notes")) {
        setError(result.error)
        return
      }
      onOpenChange(false)
      toaster.create({
        type: "error",
        title: "Couldn't receive purchase order",
        description: result.error ?? FALLBACK_ERROR,
      })
      onReceived()
      return
    }

    toaster.create({ type: "success", title: "Purchase order received" })
    onOpenChange(false)
    onReceived()
  }

  const supplier = detail ? suppliersById.get(detail.supplier_id) : undefined

  return (
    <DialogRoot
      open={open}
      onOpenChange={(details) => !details.open && onOpenChange(false)}
    >
      <DialogContent>
        <DialogCloseTrigger />
        <DialogHeader>
          <DialogTitle>Receive purchase order</DialogTitle>
        </DialogHeader>
        <DialogBody>
          {loadingDetail ? (
            <Center py="6">
              <Spinner />
            </Center>
          ) : loadError ? (
            <Alert status="error" title="Couldn't load purchase order">
              {loadError}
            </Alert>
          ) : (
            <Stack gap="4">
              {error && (
                <Alert status="error" title="Couldn't receive purchase order">
                  {error}
                </Alert>
              )}
              <Text>
                This will receive{" "}
                <strong>{detail?.items.length ?? 0} item(s)</strong> in full
                from{" "}
                <strong>{supplier ? supplier.name : detail?.supplier_id}</strong>
                , adding each ordered quantity to stock.
              </Text>
              <Field label="Notes" optionalText="(optional)">
                <Textarea
                  value={notes}
                  onChange={(event) => setNotes(event.target.value)}
                  maxLength={500}
                />
              </Field>
            </Stack>
          )}
        </DialogBody>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button
            loading={submitting}
            disabled={loadingDetail || !!loadError}
            onClick={handleConfirm}
          >
            Receive
          </Button>
        </DialogFooter>
      </DialogContent>
    </DialogRoot>
  )
}
