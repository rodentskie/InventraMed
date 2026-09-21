import { EmotionRegistry } from "@inventramed/snippets/emotion-registry"
import { Provider } from "@inventramed/snippets/provider"

export default function RootLayout(props: { children: React.ReactNode }) {
  return (
    <html suppressHydrationWarning>
      <body>
        <EmotionRegistry>
          <Provider>{props.children}</Provider>
        </EmotionRegistry>
      </body>
    </html>
  )
}