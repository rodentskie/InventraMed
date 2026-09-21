"use client"

import { IconButton } from "@chakra-ui/react"
import {
  DrawerBody,
  DrawerCloseTrigger,
  DrawerContent,
  DrawerHeader,
  DrawerRoot,
  DrawerTitle,
  DrawerTrigger,
} from "@inventramed/snippets/drawer"
import { useState } from "react"
import { LuMenu } from "react-icons/lu"
import { NavLinks } from "./NavLinks"

export function MobileNav() {
  const [open, setOpen] = useState(false)

  return (
    <DrawerRoot
      placement="start"
      size="xs"
      open={open}
      onOpenChange={(event) => setOpen(event.open)}
    >
      <DrawerTrigger asChild>
        <IconButton
          hideFrom="md"
          variant="ghost"
          size="sm"
          aria-label="Open menu"
        >
          <LuMenu />
        </IconButton>
      </DrawerTrigger>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>Menu</DrawerTitle>
        </DrawerHeader>
        <DrawerBody>
          <NavLinks onNavigate={() => setOpen(false)} />
        </DrawerBody>
        <DrawerCloseTrigger />
      </DrawerContent>
    </DrawerRoot>
  )
}
