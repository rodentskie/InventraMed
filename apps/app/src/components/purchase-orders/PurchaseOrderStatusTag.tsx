import { Tag } from "@inventramed/snippets/tag"
import type { PurchaseOrderStatus } from "../../types/purchase-order"

const STATUS_LABEL: Record<PurchaseOrderStatus, string> = {
  draft: "Draft",
  ordered: "Ordered",
  partially_received: "Partially Received",
  received: "Received",
  cancelled: "Cancelled",
}

const STATUS_COLOR: Record<PurchaseOrderStatus, string> = {
  draft: "gray",
  ordered: "blue",
  partially_received: "orange",
  received: "green",
  cancelled: "red",
}

interface PurchaseOrderStatusTagProps {
  status: PurchaseOrderStatus
}

export function PurchaseOrderStatusTag({ status }: PurchaseOrderStatusTagProps) {
  return <Tag colorPalette={STATUS_COLOR[status]}>{STATUS_LABEL[status]}</Tag>
}

// Only draft/ordered POs can be received in this pass — see
// @context/features/10-purchase-orders.spec.md.
export function isPurchaseOrderReceivable(status: PurchaseOrderStatus): boolean {
  return status === "draft" || status === "ordered"
}
