import { Heading } from "@chakra-ui/react"
import type { Metadata } from "next"

export const metadata: Metadata = {
  title: "Dashboard | InventraMed",
}

export default function HomePage() {
  return <Heading size="lg">Dashboard</Heading>
}
