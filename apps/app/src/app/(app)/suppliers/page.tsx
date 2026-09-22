import type { Metadata } from "next"
import { SuppliersPageClient } from "../../../components/suppliers/SuppliersPageClient"

export const metadata: Metadata = {
  title: "Suppliers | InventraMed",
}

export default function SuppliersPage() {
  return <SuppliersPageClient />
}
