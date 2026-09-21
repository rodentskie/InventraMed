"use client"

import { Stack, Text } from "@chakra-ui/react"
import { usePathname } from "next/navigation"
import { NAV_ITEMS } from "../../lib/nav"
import { NavGroup } from "./NavGroup"
import { NavItemLink } from "./NavItemLink"

interface NavLinksProps {
  onNavigate?: () => void
}

export function NavLinks({ onNavigate }: NavLinksProps) {
  const pathname = usePathname()

  return (
    <Stack gap="1" as="nav" aria-label="Main">
      <Text
        px="3"
        mb="1"
        fontSize="xs"
        fontWeight="semibold"
        letterSpacing="wider"
        textTransform="uppercase"
        color="fg.muted"
      >
        Workspace
      </Text>
      {NAV_ITEMS.map((item) =>
        item.children ? (
          <NavGroup
            key={item.href}
            item={item}
            pathname={pathname}
            onNavigate={onNavigate}
          />
        ) : (
          <NavItemLink
            key={item.href}
            item={item}
            active={pathname === item.href}
            onNavigate={onNavigate}
          />
        ),
      )}
    </Stack>
  )
}
