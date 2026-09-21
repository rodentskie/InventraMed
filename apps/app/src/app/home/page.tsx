import { Center, Heading } from "@chakra-ui/react"
import type { Metadata } from "next"

export const metadata: Metadata = {
  title: "Home | InventraMed",
}

export default function HomePage() {
  return (
    <Center minH="100vh">
      <Heading>Home</Heading>
    </Center>
  )
}
