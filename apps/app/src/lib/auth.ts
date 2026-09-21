export const ACCESS_TOKEN_COOKIE = "access_token"
export const REFRESH_TOKEN_COOKIE = "refresh_token"

const FALLBACK_INITIAL = "?"

interface TokenClaims {
  payload?: { email?: string }
}

// Display only: the signature is not verified here, the API verifies the
// token on every call it receives.
export function getEmailFromToken(token: string | undefined): string | null {
  const segment = token?.split(".")[1]
  if (!segment) return null

  try {
    const claims = JSON.parse(
      Buffer.from(segment, "base64url").toString("utf8"),
    ) as TokenClaims
    return claims.payload?.email ?? null
  } catch {
    return null
  }
}

export function getInitial(email: string | null): string {
  const first = email?.trim().charAt(0)
  return first ? first.toUpperCase() : FALLBACK_INITIAL
}
