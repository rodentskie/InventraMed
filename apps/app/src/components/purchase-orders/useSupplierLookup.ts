"use client"

import { useEffect, useMemo, useState } from "react"
import { listSuppliers } from "../../actions/suppliers"
import type { Supplier } from "../../types/supplier"

// The API's max page size (`limit` maxes out at 100). This is a best-effort
// lookup for resolving supplier_id -> name: it only covers the first 100
// active suppliers and excludes deleted ones. A purchase order referencing a
// supplier outside that set falls back to showing its raw supplier_id.
const SUPPLIER_LOOKUP_LIMIT = 100

export function useSupplierLookup(): Map<string, Supplier> {
  const [suppliers, setSuppliers] = useState<Supplier[]>([])

  useEffect(() => {
    listSuppliers({ limit: SUPPLIER_LOOKUP_LIMIT, offset: 0 }).then((result) => {
      if (result.success && result.data) setSuppliers(result.data.data)
    })
  }, [])

  return useMemo(
    () => new Map(suppliers.map((supplier) => [supplier.id, supplier])),
    [suppliers],
  )
}
