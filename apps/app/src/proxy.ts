import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"
import { ACCESS_TOKEN_COOKIE, REFRESH_TOKEN_COOKIE } from "./lib/auth"

const LOGIN_ROUTE = "/"

// Optimistic check: only the presence of the token cookies is verified here.
export function proxy(request: NextRequest) {
  const hasTokens = [ACCESS_TOKEN_COOKIE, REFRESH_TOKEN_COOKIE].every(
    (name) => request.cookies.get(name)?.value,
  )
  if (hasTokens) return NextResponse.next()

  return NextResponse.redirect(new URL(LOGIN_ROUTE, request.url))
}

export const config = {
  matcher: [
    "/home",
    "/home/:path*",
    "/medicines",
    "/medicines/:path*",
    "/inventory-entries",
    "/inventory-entries/:path*",
    "/suppliers",
    "/suppliers/:path*",
  ],
}
