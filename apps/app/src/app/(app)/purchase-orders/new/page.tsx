import type { Metadata } from "next"
import { PurchaseOrderFormPage } from "../../../../components/purchase-orders/PurchaseOrderFormPage"

export const metadata: Metadata = {
  title: "New Purchase Order | InventraMed",
}

export default function NewPurchaseOrderPage() {
  return <PurchaseOrderFormPage />
}
