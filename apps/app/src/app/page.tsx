import { Card, Center, Flex, Separator, Stack, Text } from "@chakra-ui/react"
import { ColorModeButton } from "@inventramed/snippets/color-mode"
import type { Metadata } from "next"
import { LoginForm } from "../components/auth/LoginForm"
import { BrandLogo } from "../components/brand/BrandLogo"

export const metadata: Metadata = {
  title: "Sign in | InventraMed",
}

export default function Index() {
  return (
    <Center
      minH="100vh"
      px="4"
      py="8"
      bg="bg.subtle"
      backgroundImage="radial-gradient(circle, {colors.border} 1px, transparent 1px)"
      backgroundSize="24px 24px"
    >
      <Stack width="full" maxW="sm" gap="6" align="center">
        <BrandLogo />
        <Card.Root width="full" bg="bg.panel">
          <Card.Body gap="6" p="8">
            <Flex justify="space-between" align="flex-start">
              <Stack gap="1">
                <Card.Title fontSize="xl">Sign in</Card.Title>
                <Card.Description>Medicine inventory management</Card.Description>
              </Stack>
              <ColorModeButton />
            </Flex>
            <LoginForm />
            <Separator />
            <Text fontSize="sm" color="fg.muted" textAlign="center">
              Access is limited to InventraMed staff accounts.
            </Text>
          </Card.Body>
        </Card.Root>
        <Text fontSize="xs" color="fg.muted">
          © {new Date().getFullYear()} InventraMed · Internal inventory system
        </Text>
      </Stack>
    </Center>
  )
}
