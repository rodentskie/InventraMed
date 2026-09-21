import { EmotionRegistry } from "@inventramed/snippets/emotion-registry"
import { Provider } from "@inventramed/snippets/provider"
import { Toaster } from "@inventramed/snippets/toaster"

export const metadata = {
  title: "Test"
}

export default function RootLayout(props: { children: React.ReactNode }) {
  return (
    <html suppressHydrationWarning>
      <body>
        <EmotionRegistry>
          <Provider defaultTheme="dark">
            {props.children}
            <Toaster />
          </Provider>
        </EmotionRegistry>
      </body>
    </html>
  )
}