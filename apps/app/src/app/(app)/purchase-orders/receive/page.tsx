import { Center, Spinner } from "@chakra-ui/react"
import type { Metadata } from "next"
import { Suspense } from "react"
import { ReceivePurchaseOrderPageClient } from "../../../../components/purchase-orders/ReceivePurchaseOrderPageClient"

export const metadata: Metadata = {
  title: "Receive Purchase Order | InventraMed",
}

export default function ReceivePurchaseOrderPage() {
  return (
    <Suspense
      fallback={
        <Center py="12">
          <Spinner />
        </Center>
      }
    >
      <ReceivePurchaseOrderPageClient />
    </Suspense>
  )
}
