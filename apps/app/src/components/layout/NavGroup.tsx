"use client"

import { Button, Collapsible, Stack } from "@chakra-ui/react"
import { LuChevronDown } from "react-icons/lu"
import type { NavItem } from "../../lib/nav"
import { NavItemLink } from "./NavItemLink"

interface NavGroupProps {
  item: NavItem
  pathname: string
  onNavigate?: () => void
}

export function NavGroup({ item, pathname, onNavigate }: NavGroupProps) {
  return (
    <Collapsible.Root defaultOpen={pathname.startsWith(item.href)}>
      <Collapsible.Trigger asChild>
        <Button
          variant="ghost"
          size="sm"
          width="full"
          justifyContent="space-between"
          px="3"
          fontWeight="normal"
          color="fg.muted"
          _hover={{ bg: "bg.muted", color: "fg" }}
        >
          {item.label}
          <Collapsible.Indicator
            display="inline-flex"
            transition="transform 0.2s"
            _open={{ transform: "rotate(180deg)" }}
          >
            <LuChevronDown />
          </Collapsible.Indicator>
        </Button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <Stack gap="1" mt="1">
          {item.children?.map((child) => (
            <NavItemLink
              key={child.href}
              item={child}
              active={pathname === child.href}
              nested
              onNavigate={onNavigate}
            />
          ))}
        </Stack>
      </Collapsible.Content>
    </Collapsible.Root>
  )
}
