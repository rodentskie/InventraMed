"use server"

import { cookies } from "next/headers"
import { ACCESS_TOKEN_COOKIE } from "../lib/auth"
import { toLocationMessage } from "../lib/live"
import type { ActionResult } from "../types/action"
import type { LocationMessage } from "../types/live"
import type {
  CreateMedicineInput,
  ErrorResponse,
  Medicine,
  MedicineListResponse,
  UpdateMedicineInput,
} from "../types/medicine"

const GENERIC_ERROR = "Something went wrong. Please try again."
// These statuses carry a meaningful `error` message from the API; others
// (e.g. 500) may not always return JSON, so they fall back to a generic one.
const MESSAGE_STATUSES = [400, 401, 404, 409]

interface ListMedicinesParams {
  limit: number
  offset: number
  name?: string
}

async function readErrorMessage(res: Response): Promise<string> {
  if (!MESSAGE_STATUSES.includes(res.status)) return GENERIC_ERROR

  const body = (await res.json()) as ErrorResponse
  return body.error || GENERIC_ERROR
}

async function medicinesUrl(path = ""): Promise<string> {
  const apiPrefix = process.env.API_PREFIX ?? ""
  return `${process.env.API_URL}${apiPrefix}/medicines${path}`
}

async function authHeaders(): Promise<HeadersInit> {
  const cookieStore = await cookies()
  const token = cookieStore.get(ACCESS_TOKEN_COOKIE)?.value
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export async function listMedicines(
  params: ListMedicinesParams,
): Promise<ActionResult<MedicineListResponse>> {
  try {
    const query = new URLSearchParams({
      limit: String(params.limit),
      offset: String(params.offset),
    })
    if (params.name) query.set("name", params.name)

    const res = await fetch(`${await medicinesUrl()}?${query}`, {
      headers: await authHeaders(),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    return { success: true, data: (await res.json()) as MedicineListResponse, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}

export async function createMedicine(
  input: CreateMedicineInput,
): Promise<ActionResult<Medicine>> {
  try {
    const res = await fetch(await medicinesUrl(), {
      method: "POST",
      headers: { "Content-Type": "application/json", ...(await authHeaders()) },
      body: JSON.stringify(input),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    const body = (await res.json()) as { data: Medicine }
    return { success: true, data: body.data, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}

export async function getMedicineByBarcode(
  barcode: string,
): Promise<ActionResult<Medicine>> {
  try {
    const res = await fetch(await medicinesUrl(`/barcode/${encodeURIComponent(barcode)}`), {
      headers: await authHeaders(),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    const body = (await res.json()) as { data: Medicine }
    return { success: true, data: body.data, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}

// The LED state of every placed medicine, from the public
// GET /medicines/locations. Invalid items are dropped.
export async function getMedicineLocations(): Promise<ActionResult<LocationMessage[]>> {
  try {
    const res = await fetch(await medicinesUrl("/locations"), { cache: "no-store" })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    const body = (await res.json()) as unknown
    const messages = Array.isArray(body)
      ? body.map(toLocationMessage).filter((m): m is LocationMessage => m !== null)
      : []
    return { success: true, data: messages, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}

export async function updateMedicine(
  id: string,
  input: UpdateMedicineInput,
): Promise<ActionResult<null>> {
  try {
    const res = await fetch(await medicinesUrl(`/${id}`), {
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

export async function deleteMedicine(id: string): Promise<ActionResult<null>> {
  try {
    const res = await fetch(await medicinesUrl(`/${id}`), {
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
