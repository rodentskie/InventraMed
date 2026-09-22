export interface Supplier {
  id: string
  name: string
  contact_name: string | null
  email: string | null
  phone: string | null
  address: string | null
  created_at: string
  updated_at: string
}

export interface SupplierListResponse {
  data: Supplier[]
  total: number
  limit: number
  offset: number
}

export interface CreateSupplierInput {
  name: string
  contact_name: string | null
  email: string | null
  phone: string | null
  address: string | null
}

export interface UpdateSupplierInput {
  name: string
  contact_name: string | null
  email: string | null
  phone: string | null
  address: string | null
}
