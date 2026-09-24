import type { Metadata } from "next"
import { LivePageClient } from "../../../components/live/LivePageClient"

export const metadata: Metadata = {
  title: "Live View | InventraMed",
}

export default function LivePage() {
  return <LivePageClient />
}
