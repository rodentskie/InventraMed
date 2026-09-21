import { Center, HStack, Text } from "@chakra-ui/react"

export function BrandLogo() {
  return (
    <HStack gap="3">
      <Center boxSize="9" rounded="lg" bg="fg" color="bg" fontWeight="bold">
        IM
      </Center>
      <Text fontSize="xl" fontWeight="semibold">
        InventraMed
      </Text>
    </HStack>
  )
}
