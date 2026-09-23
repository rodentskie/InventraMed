import type { Metadata } from "next"
import { PurchaseOrderDetail } from "../../../../components/purchase-orders/PurchaseOrderDetail"

export const metadata: Metadata = {
  title: "Purchase Order | InventraMed",
}

interface PurchaseOrderPageProps {
  params: Promise<{ id: string }>
}

export default async function PurchaseOrderPage({ params }: PurchaseOrderPageProps) {
  const { id } = await params
  return <PurchaseOrderDetail id={id} />
}
