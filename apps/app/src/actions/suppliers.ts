"use server"

import { cookies } from "next/headers"
import { ACCESS_TOKEN_COOKIE } from "../lib/auth"
import type { ActionResult } from "../types/action"
import type { ErrorResponse } from "../types/medicine"
import type {
  CreateSupplierInput,
  Supplier,
  SupplierListResponse,
  UpdateSupplierInput,
} from "../types/supplier"

const GENERIC_ERROR = "Something went wrong. Please try again."
// These statuses carry a meaningful `error` message from the API; others
// (e.g. 500) may not always return JSON, so they fall back to a generic one.
const MESSAGE_STATUSES = [400, 401, 404, 409]

interface ListSuppliersParams {
  limit: number
  offset: number
  name?: string
}

async function readErrorMessage(res: Response): Promise<string> {
  if (!MESSAGE_STATUSES.includes(res.status)) return GENERIC_ERROR

  const body = (await res.json()) as ErrorResponse
  return body.error || GENERIC_ERROR
}

async function suppliersUrl(path = ""): Promise<string> {
  const apiPrefix = process.env.API_PREFIX ?? ""
  return `${process.env.API_URL}${apiPrefix}/suppliers${path}`
}

async function authHeaders(): Promise<HeadersInit> {
  const cookieStore = await cookies()
  const token = cookieStore.get(ACCESS_TOKEN_COOKIE)?.value
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export async function listSuppliers(
  params: ListSuppliersParams,
): Promise<ActionResult<SupplierListResponse>> {
  try {
    const query = new URLSearchParams({
      limit: String(params.limit),
      offset: String(params.offset),
    })
    if (params.name) query.set("name", params.name)

    const res = await fetch(`${await suppliersUrl()}?${query}`, {
      headers: await authHeaders(),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    return { success: true, data: (await res.json()) as SupplierListResponse, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}

export async function createSupplier(
  input: CreateSupplierInput,
): Promise<ActionResult<Supplier>> {
  try {
    const res = await fetch(await suppliersUrl(), {
      method: "POST",
      headers: { "Content-Type": "application/json", ...(await authHeaders()) },
      body: JSON.stringify(input),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    const body = (await res.json()) as { data: Supplier }
    return { success: true, data: body.data, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}

export async function updateSupplier(
  id: string,
  input: UpdateSupplierInput,
): Promise<ActionResult<null>> {
  try {
    const res = await fetch(await suppliersUrl(`/${id}`), {
      method: "PUT",
      headers: { "Content-Type": "application/json", ...(await authHeaders()) },
      body: JSON.stringify(input),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    return { success: true, data: null, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}

export async function deleteSupplier(id: string): Promise<ActionResult<null>> {
  try {
    const res = await fetch(await suppliersUrl(`/${id}`), {
      method: "DELETE",
      headers: await authHeaders(),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    return { success: true, data: null, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}
