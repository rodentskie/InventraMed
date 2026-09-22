import { Box, Flex } from "@chakra-ui/react"
import { cookies } from "next/headers"
import { SideNav } from "../../components/layout/SideNav"
import { TopNav } from "../../components/layout/TopNav"
import {
  ACCESS_TOKEN_COOKIE,
  getEmailFromToken,
  getInitial,
} from "../../lib/auth"

export default async function AppLayout(props: {
  children: React.ReactNode
}) {
  const cookieStore = await cookies()
  const email = getEmailFromToken(cookieStore.get(ACCESS_TOKEN_COOKIE)?.value)

  return (
    <Flex direction="column" minH="100vh">
      <TopNav initial={getInitial(email)} />
      <Flex flex="1">
        <SideNav />
        <Box as="main" flex="1" minWidth="0" p={{ base: "4", md: "8" }}>
          {props.children}
        </Box>
      </Flex>
    </Flex>
  )
}
