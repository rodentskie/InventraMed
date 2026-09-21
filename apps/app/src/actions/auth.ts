"use server"

import { cookies } from "next/headers"
import { ACCESS_TOKEN_COOKIE, REFRESH_TOKEN_COOKIE } from "../lib/auth"
import type { ActionResult } from "../types/action"
import type { ErrorResponse, LoginInput, TokenResponse } from "../types/auth"

const GENERIC_ERROR = "Something went wrong. Please try again."
const CLIENT_ERROR_STATUSES = [400, 401]

async function readErrorMessage(res: Response): Promise<string> {
  if (!CLIENT_ERROR_STATUSES.includes(res.status)) return GENERIC_ERROR

  const body = (await res.json()) as ErrorResponse
  return body.error || GENERIC_ERROR
}

async function setAuthCookies(tokens: TokenResponse): Promise<void> {
  const cookieStore = await cookies()
  const options = {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
  } as const

  cookieStore.set(ACCESS_TOKEN_COOKIE, tokens.access_token, options)
  cookieStore.set(REFRESH_TOKEN_COOKIE, tokens.refresh_token, options)
}

export async function login(input: LoginInput): Promise<ActionResult<null>> {
  const email = input.email.trim()
  if (!email || !input.password) {
    return {
      success: false,
      data: null,
      error: "Email and password are required",
    }
  }

  try {
    const apiPrefix = process.env.API_PREFIX ?? ""
    const res = await fetch(`${process.env.API_URL}${apiPrefix}/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password: input.password }),
      cache: "no-store",
    })
    if (!res.ok) {
      return { success: false, data: null, error: await readErrorMessage(res) }
    }

    await setAuthCookies((await res.json()) as TokenResponse)
    return { success: true, data: null, error: null }
  } catch {
    return { success: false, data: null, error: GENERIC_ERROR }
  }
}
