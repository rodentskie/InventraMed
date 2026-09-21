import { Flex, HStack } from "@chakra-ui/react"
import { Avatar } from "@inventramed/snippets/avatar"
import { ColorModeButton } from "@inventramed/snippets/color-mode"
import { AppIcon } from "../brand/AppIcon"
import { MobileNav } from "./MobileNav"

interface TopNavProps {
  initial: string
}

export function TopNav({ initial }: TopNavProps) {
  return (
    <Flex
      as="header"
      position="sticky"
      top="0"
      zIndex="sticky"
      height="14"
      px="4"
      align="center"
      justify="space-between"
      borderBottomWidth="1px"
      bg="bg"
    >
      <HStack gap="2">
        <MobileNav />
        <AppIcon />
      </HStack>
      <HStack gap="2">
        <Avatar size="sm" fallback={initial} />
        <ColorModeButton />
      </HStack>
    </Flex>
  )
}
