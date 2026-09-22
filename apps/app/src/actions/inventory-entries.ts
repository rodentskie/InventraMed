"use server"

import { cookies } from "next/headers"
import { ACCESS_TOKEN_COOKIE } from "../lib/auth"
import type { ActionResult } from "../types/action"
import type {
  CreateInventoryEntryInput,
  InventoryEntry,
  InventoryEntryListResponse,
} from "../types/inventory-entry"
import type { ErrorResponse } from "../types/medicine"

const GENERIC_ERROR = "Something went wrong. Please try again."
// These statuses carry a meaningful `error` message from the API; others
// (e.g. 500) may not always return JSON, so they fall back to a generic one.
const MESSAGE_STATUSES = [400, 401, 404, 409]

interface ListInventoryEntriesParams {
  limit: number
  offset: number
}

async function readErrorMessage(res: Response): Promise<string> {
  if (!MESSAGE_STATUSES.includes(res.status)) return GENERIC_ERROR

  const body = (await res.json()) as ErrorResponse
  return body.error || GENERIC_ERROR
}

async function inventoryEntriesUrl(path = ""): Promise<string> {
  const apiPrefix = process.env.API_PREFIX ?? ""
  return `${process.env.API_URL}${apiPrefix}/inventory-entries${path}`
}

async function authHeaders(): Promise<HeadersInit> {
  const cookieStore = await cookies()
  const token = cookieStore.get(ACCESS_TOKEN_COOKIE)?.value
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export async function listInventoryEntries(
  params: ListInventoryEntriesParams,
): Promise<ActionResult<InventoryEntryListResponse>> {
  try {
    const query = new URLSearchParams({
      limit: String(params.limit),
      offset: String(params.offset),
    })
    const res = await fetch(`${await inventoryEntriesUrl()}?${query}`, {
      headers: await authHeaders(),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    return {
      success: true,
      data: (await res.json()) as InventoryEntryListResponse,
      error: null,
    }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}

export async function createInventoryEntry(
  input: CreateInventoryEntryInput,
): Promise<ActionResult<InventoryEntry>> {
  try {
    const res = await fetch(await inventoryEntriesUrl(), {
      method: "POST",
      headers: { "Content-Type": "application/json", ...(await authHeaders()) },
      body: JSON.stringify(input),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    const body = (await res.json()) as { data: InventoryEntry }
    return { success: true, data: body.data, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}
