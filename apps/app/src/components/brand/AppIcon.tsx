"use client"

import { HStack, Image, Text } from "@chakra-ui/react"
import { useEffect, useRef, useState } from "react"
import { BrandLogo } from "./BrandLogo"

const ICON_SRC = "/favicon.ico"

export function AppIcon() {
  const imageRef = useRef<HTMLImageElement>(null)
  const [failed, setFailed] = useState(false)

  // The error event can fire before hydration, so check the loaded state too.
  useEffect(() => {
    const image = imageRef.current
    if (image?.complete && image.naturalWidth === 0) setFailed(true)
  }, [])

  if (failed) return <BrandLogo />

  return (
    <HStack gap="3">
      <Image
        ref={imageRef}
        src={ICON_SRC}
        alt=""
        boxSize="9"
        rounded="lg"
        onError={() => setFailed(true)}
      />
      <Text fontSize="xl" fontWeight="semibold">
        InventraMed
      </Text>
    </HStack>
  )
}
