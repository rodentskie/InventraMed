export interface Medicine {
  id: string
  name: string
  barcode: string
  batch_number: string | null
  expiration_date: string
  quantity: number
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
}

export interface UpdateMedicineInput {
  name: string
  barcode: string
  batch_number: string | null
  expiration_date: string
}

export interface ErrorResponse {
  error: string
}
