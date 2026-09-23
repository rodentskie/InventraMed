"use client"

import { Flex, Heading, Stack } from "@chakra-ui/react"
import { useCallback, useEffect, useRef, useState } from "react"
import { getMedicineByBarcode } from "../../actions/medicines"
import type { Medicine } from "../../types/medicine"
import { BarcodeCameraScanner } from "./BarcodeCameraScanner"
import { ScanResultCard } from "./ScanResultCard"

const FALLBACK_ERROR = "Something went wrong. Please try again."
const DEFAULT_COOLDOWN_SECONDS = 5
const COOLDOWN_MS =
  Number(process.env.NEXT_PUBLIC_SCANNER_COOLDOWN_SECONDS) * 1000 ||
  DEFAULT_COOLDOWN_SECONDS * 1000

export function ScannerPageClient() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [medicine, setMedicine] = useState<Medicine | null>(null)
  const [scannedBarcode, setScannedBarcode] = useState<string | null>(null)
  // Also stops the camera from re-triggering lookups until the cooldown ends.
  const [paused, setPaused] = useState(false)
  const cooldownRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => () => clearTimeout(cooldownRef.current), [])

  const lookup = useCallback(async (barcode: string) => {
    clearTimeout(cooldownRef.current)
    setPaused(true)
    setScannedBarcode(barcode)
    setMedicine(null)
    setError(null)
    setLoading(true)

    const result = await getMedicineByBarcode(barcode)

    setLoading(false)
    if (!result.success || !result.data) {
      setError(result.error ?? FALLBACK_ERROR)
    } else {
      setMedicine(result.data)
    }

    cooldownRef.current = setTimeout(() => setPaused(false), COOLDOWN_MS)
  }, [])

  const hasResult = loading || error != null || medicine != null

  return (
    <Stack gap="6">
      <Heading size="lg">Scanner</Heading>

      <Flex gap="8" wrap="wrap" align="flex-start">
        <BarcodeCameraScanner paused={paused} onScan={lookup} />
        {hasResult && (
          <ScanResultCard
            loading={loading}
            error={error}
            scannedBarcode={scannedBarcode}
            medicine={medicine}
          />
        )}
      </Flex>
    </Stack>
  )
}
