"use client"

import { Button, Field, Input, Stack } from "@chakra-ui/react"
import { PasswordInput } from "@inventramed/snippets/password-input"
import { toaster } from "@inventramed/snippets/toaster"
import { useRouter } from "next/navigation"
import { useState } from "react"
import { login } from "../../actions/auth"

const HOME_ROUTE = "/home"
const FALLBACK_ERROR = "Something went wrong. Please try again."

export function LoginForm() {
  const router = useRouter()
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setLoading(true)

    const result = await login({ email, password })
    if (!result.success) {
      toaster.create({
        type: "error",
        title: "Sign in failed",
        description: result.error ?? FALLBACK_ERROR,
      })
      setLoading(false)
      return
    }

    toaster.create({
      type: "success",
      title: "Signed in",
      description: "Redirecting to your dashboard…",
    })
    router.push(HOME_ROUTE)
  }

  return (
    <form onSubmit={handleSubmit} noValidate>
      <Stack gap="4">
        <Field.Root>
          <Field.Label>Email address</Field.Label>
          <Input
            type="email"
            name="email"
            autoComplete="email"
            placeholder="name@example.com"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </Field.Root>
        <Field.Root>
          <Field.Label>Password</Field.Label>
          <PasswordInput
            name="password"
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </Field.Root>
        <Button type="submit" width="full" mt="2" loading={loading}>
          Sign in
        </Button>
      </Stack>
    </form>
  )
}
