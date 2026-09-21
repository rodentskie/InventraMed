import { Code, HStack } from "@chakra-ui/react"
import { ColorModeButton } from "@inventramed/snippets/color-mode"

export default function Index() {
  return (
    <HStack p="4" gap="4">
      <Code>{`console.log("Hello, world!")`}</Code>
      <ColorModeButton />
    </HStack>
  )
}
