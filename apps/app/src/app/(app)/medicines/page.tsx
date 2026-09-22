import type { Metadata } from "next"
import { MedicinesPageClient } from "../../../components/medicines/MedicinesPageClient"

export const metadata: Metadata = {
  title: "Medicines | InventraMed",
}

export default function MedicinesPage() {
  return <MedicinesPageClient />
}
