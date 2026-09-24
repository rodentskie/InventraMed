import { Box, Flex } from "@chakra-ui/react"
import { ColorModeButton } from "@inventramed/snippets/color-mode"
import type { Metadata } from "next"
import Link from "next/link"
import { connection } from "next/server"
import { BrandLogo } from "../../components/brand/BrandLogo"
import { ScannerPageClient } from "../../components/scanner/ScannerPageClient"

export const metadata: Metadata = {
  title: "Scanner | InventraMed",
}

// Public page, outside the `(app)` guarded route group: no login required,
// so it doesn't get the authenticated SideNav/TopNav chrome.
export default async function ScannerPage() {
  // Renders per request, so WS_SERVER is read at runtime and not inlined at
  // build time. The browser connects to it, but it needs no NEXT_PUBLIC_.
  await connection()
  const wsUrl = process.env.WS_SERVER ?? ""

  return (
    <Box minH="100vh">
      <Flex
        as="header"
        justify="space-between"
        align="center"
        px={{ base: "4", md: "8" }}
        py="4"
        borderBottomWidth="1px"
      >
        <Link href="/">
          <BrandLogo />
        </Link>
        <ColorModeButton />
      </Flex>
      <Box as="main" p={{ base: "4", md: "8" }}>
        <ScannerPageClient wsUrl={wsUrl} />
      </Box>
    </Box>
  )
}
