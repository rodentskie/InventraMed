import { Box } from "@chakra-ui/react"
import { NavLinks } from "./NavLinks"

export function SideNav() {
  return (
    <Box
      as="aside"
      hideBelow="md"
      width="64"
      flexShrink="0"
      p="4"
      borderEndWidth="1px"
      bg="bg.subtle"
    >
      <NavLinks />
    </Box>
  )
}
