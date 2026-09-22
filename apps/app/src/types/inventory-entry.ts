export type InventoryEntryDirection = "addition" | "subtraction"

export interface InventoryEntryCountedBy {
  id: string
  name: string
}

export interface InventoryEntry {
  id: string
  medicine_id: string
  direction: InventoryEntryDirection
  quantity: number
  reason: string
  counted_by: InventoryEntryCountedBy
  notes: string | null
  created_at: string
}

export interface InventoryEntryListResponse {
  data: InventoryEntry[]
  total: number
  limit: number
  offset: number
}

export interface CreateInventoryEntryInput {
  medicine_id: string
  direction: InventoryEntryDirection
  quantity: number
  reason: string
  notes: string | null
}
