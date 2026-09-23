export type PurchaseOrderStatus =
  | "draft"
  | "ordered"
  | "partially_received"
  | "received"
  | "cancelled"

export interface PurchaseOrder {
  id: string
  supplier_id: string
  status: PurchaseOrderStatus
  order_date: string
  expected_date: string | null
  created_by: string
  notes: string | null
  created_at: string
  updated_at: string
}

export interface PurchaseOrderItem {
  id: string
  medicine_id: string
  quantity_ordered: number
  created_at: string
}

export interface PurchaseOrderReceiptItem {
  id: string
  purchase_order_item_id: string
  quantity_received: number
  quantity_damaged: number
  quantity_returned: number
  notes: string | null
  created_at: string
}

export interface PurchaseOrderReceipt {
  id: string
  purchase_order_id: string
  received_by: string
  received_at: string
  notes: string | null
  created_at: string
  items: PurchaseOrderReceiptItem[]
}

export interface PurchaseOrderDetail extends PurchaseOrder {
  items: PurchaseOrderItem[]
  receipts: PurchaseOrderReceipt[]
}

export interface PurchaseOrderListResponse {
  data: PurchaseOrder[]
  total: number
  limit: number
  offset: number
}

export interface CreatePurchaseOrderItemInput {
  medicine_id: string
  quantity_ordered: number
}

export interface CreatePurchaseOrderInput {
  supplier_id: string
  order_date: string
  expected_date: string | null
  notes: string | null
  items: CreatePurchaseOrderItemInput[]
}

export interface ReceivePurchaseOrderInput {
  notes: string | null
}
