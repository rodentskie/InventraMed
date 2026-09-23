"use client"

import { Box, Stack, Text } from "@chakra-ui/react"
import { Alert } from "@inventramed/snippets/alert"
import dynamic from "next/dynamic"
import { useCallback, useEffect, useRef, useState } from "react"
import type { DetectedBarcode } from "react-barcode-scanner"

// Medicine barcodes are 1D — no QR codes are generated for them.
const SCAN_FORMATS = ["code_128", "ean_13", "ean_8", "upc_a", "upc_e"]

// Loaded client-side only: the component (and its polyfill) touch
// `navigator`/camera APIs that don't exist during SSR.
const BarcodeScanner = dynamic(
  async () => {
    await import("react-barcode-scanner/polyfill")
    const { BarcodeScanner: Scanner } = await import("react-barcode-scanner")
    return Scanner
  },
  { ssr: false },
)

interface BarcodeCameraScannerProps {
  paused: boolean
  onScan: (barcode: string) => void
}

export function BarcodeCameraScanner({ paused, onScan }: BarcodeCameraScannerProps) {
  const [cameraError, setCameraError] = useState<Error | null>(null)
  const lastCaptureRef = useRef<string | null>(null)

  // Once the cooldown resumes scanning, forget the last capture so the same
  // barcode can be picked up again instead of being silently ignored.
  useEffect(() => {
    if (!paused) lastCaptureRef.current = null
  }, [paused])

  const handleCapture = useCallback(
    (barcodes: DetectedBarcode[]) => {
      const value = barcodes[0]?.rawValue
      if (!value || value === lastCaptureRef.current) return

      lastCaptureRef.current = value
      onScan(value)
    },
    [onScan],
  )

  if (cameraError) {
    const denied = cameraError.name === "NotAllowedError"
    return (
      <Alert
        status="warning"
        title={denied ? "Camera access denied" : "Camera unavailable"}
        maxW="md"
      >
        {denied
          ? "Allow camera access in your browser settings and reload the page."
          : "The camera couldn't be started. Reload the page to try again."}
      </Alert>
    )
  }

  return (
    <Stack gap="2">
      <Box
        borderWidth="1px"
        rounded="md"
        overflow="hidden"
        bg="black"
        w="full"
        maxW="md"
        aspectRatio={4 / 3}
      >
        <BarcodeScanner
          paused={paused}
          onCapture={handleCapture}
          onCameraError={setCameraError}
          options={{ formats: SCAN_FORMATS, delay: 500 }}
          trackConstraints={{ facingMode: "environment" }}
          style={{ width: "100%", height: "100%", objectFit: "cover" }}
        />
      </Box>
      <Text fontSize="sm" color="fg.muted">
        {paused
          ? "Scanning paused — resumes automatically."
          : "Point the camera at a medicine's barcode."}
      </Text>
    </Stack>
  )
}
