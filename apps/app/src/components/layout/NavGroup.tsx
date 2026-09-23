"use client"

import { Collapsible, HStack, IconButton, Link, Stack } from "@chakra-ui/react"
import NextLink from "next/link"
import { LuChevronDown } from "react-icons/lu"
import type { NavItem } from "../../lib/nav"
import { NavItemLink } from "./NavItemLink"

interface NavGroupProps {
  item: NavItem
  pathname: string
  onNavigate?: () => void
}

export function NavGroup({ item, pathname, onNavigate }: NavGroupProps) {
  const active = pathname === item.href

  return (
    <Collapsible.Root defaultOpen={pathname.startsWith(item.href)}>
      <HStack gap="0">
        <Link
          asChild
          flex="1"
          minW="0"
          px="3"
          py="2"
          rounded="md"
          fontSize="sm"
          fontWeight={active ? "medium" : "normal"}
          color={active ? "fg" : "fg.muted"}
          bg={active ? "bg.muted" : "transparent"}
          textDecoration="none"
          _hover={{ bg: "bg.muted", color: "fg" }}
        >
          <NextLink
            href={item.href}
            aria-current={active ? "page" : undefined}
            onClick={onNavigate}
          >
            {item.label}
          </NextLink>
        </Link>
        <Collapsible.Trigger asChild>
          <IconButton
            aria-label={`Toggle ${item.label} submenu`}
            variant="ghost"
            size="sm"
            color="fg.muted"
            _hover={{ bg: "bg.muted", color: "fg" }}
          >
            <Collapsible.Indicator
              display="inline-flex"
              transition="transform 0.2s"
              _open={{ transform: "rotate(180deg)" }}
            >
              <LuChevronDown />
            </Collapsible.Indicator>
          </IconButton>
        </Collapsible.Trigger>
      </HStack>
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
