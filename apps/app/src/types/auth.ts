export interface LoginInput {
  email: string
  password: string
}

export interface TokenResponse {
  access_token: string
  refresh_token: string
}

export interface ErrorResponse {
  error: string
}
