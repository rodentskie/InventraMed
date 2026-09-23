import type { Metadata } from "next"
import { PurchaseOrdersPageClient } from "../../../components/purchase-orders/PurchaseOrdersPageClient"

export const metadata: Metadata = {
  title: "Purchase Orders | InventraMed",
}

export default function PurchaseOrdersPage() {
  return <PurchaseOrdersPageClient />
}
