import type { Metadata } from "next"
import { connection } from "next/server"
import { LivePageClient } from "../../../components/live/LivePageClient"

export const metadata: Metadata = {
  title: "Live View | InventraMed",
}

export default async function LivePage() {
  // Renders per request, so WS_SERVER is read at runtime, like /scanner.
  await connection()
  const wsUrl = process.env.WS_SERVER ?? ""

  return <LivePageClient wsUrl={wsUrl} />
}
