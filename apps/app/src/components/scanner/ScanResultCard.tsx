"use client"

import { Box, Center, Spinner } from "@chakra-ui/react"
import { Alert } from "@inventramed/snippets/alert"
import { DataListItem, DataListRoot } from "@inventramed/snippets/data-list"
import { Status } from "@inventramed/snippets/status"
import Barcode from "react-barcode"
import { locationLabel } from "../../lib/live"
import { getMedicineStatus } from "../../lib/medicine-status"
import type { Medicine } from "../../types/medicine"

const STATUS_LABEL = {
  green: "Fresh",
  yellow: "Nearing expiration",
  red: "Expired",
} as const

const STATUS_VALUE = {
  green: "success",
  yellow: "warning",
  red: "error",
} as const

const EMPTY_VALUE = "—"

interface ScanResultCardProps {
  loading: boolean
  error: string | null
  scannedBarcode: string | null
  medicine: Medicine | null
}

export function ScanResultCard({
  loading,
  error,
  scannedBarcode,
  medicine,
}: ScanResultCardProps) {
  if (loading) {
    return (
      <Center py="8">
        <Spinner />
      </Center>
    )
  }

  if (error) {
    return (
      <Alert status="error" title="Couldn't find that medicine" maxW="md">
        {scannedBarcode && `Scanned "${scannedBarcode}". `}
        {error}
      </Alert>
    )
  }

  if (!medicine) return null

  const status = getMedicineStatus(medicine.expiration_date)

  return (
    <Box borderWidth="1px" rounded="md" p="4" maxW="md">
      <DataListRoot gap="3">
        <DataListItem label="Name" value={medicine.name} />
        <DataListItem label="Barcode" value={medicine.barcode} />
        <DataListItem
          label="Batch Number"
          value={medicine.batch_number ?? EMPTY_VALUE}
        />
        <DataListItem
          label="Location"
          value={
            medicine.location != null
              ? locationLabel(medicine.location)
              : EMPTY_VALUE
          }
        />
        <DataListItem label="Expiration Date" value={medicine.expiration_date} />
        <DataListItem label="Quantity" value={medicine.quantity} />
        <DataListItem
          label="Status"
          value={<Status value={STATUS_VALUE[status]}>{STATUS_LABEL[status]}</Status>}
        />
      </DataListRoot>
      <Box mt="4" display="inline-block" rounded="sm" overflow="hidden">
        <Barcode
          value={medicine.barcode}
          format="CODE128"
          displayValue={false}
          height={60}
          background="#ffffff"
        />
      </Box>
    </Box>
  )
}
