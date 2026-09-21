"use client"

import { Link } from "@chakra-ui/react"
import NextLink from "next/link"
import type { NavItem } from "../../lib/nav"

interface NavItemLinkProps {
  item: NavItem
  active: boolean
  nested?: boolean
  onNavigate?: () => void
}

export function NavItemLink({
  item,
  active,
  nested = false,
  onNavigate,
}: NavItemLinkProps) {
  return (
    <Link
      asChild
      pe="3"
      ps={nested ? "8" : "3"}
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
  )
}
