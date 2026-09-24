export interface Medicine {
  id: string
  name: string
  barcode: string
  batch_number: string | null
  expiration_date: string
  quantity: number
  // Tray compartment (1–12), or null when not placed.
  location: number | null
  created_by: string | null
  created_at: string
  updated_at: string
}

export interface MedicineListResponse {
  data: Medicine[]
  total: number
  limit: number
  offset: number
}

export interface CreateMedicineInput {
  name: string
  barcode: string
  batch_number: string | null
  expiration_date: string
  quantity: number
  location: number | null
}

export interface UpdateMedicineInput {
  name: string
  barcode: string
  batch_number: string | null
  expiration_date: string
  // Always sent: the API clears an omitted location.
  location: number | null
}

export interface ErrorResponse {
  error: string
}
