import type { Metadata } from "next"
import { InventoryEntriesPageClient } from "../../../components/inventory-entries/InventoryEntriesPageClient"

export const metadata: Metadata = {
  title: "Inventory Entries | InventraMed",
}

export default function InventoryEntriesPage() {
  return <InventoryEntriesPageClient />
}
