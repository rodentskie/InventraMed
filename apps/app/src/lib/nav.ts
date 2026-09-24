export interface NavItem {
  label: string
  href: string
  children?: NavItem[]
}

export const NAV_ITEMS: NavItem[] = [
  { label: "Dashboard", href: "/home" },
  { label: "Scanner", href: "/scanner" },
  { label: "Live View", href: "/live" },
  { label: "Medicines", href: "/medicines" },
  { label: "Inventory Entries", href: "/inventory-entries" },
  { label: "Suppliers", href: "/suppliers" },
  {
    label: "Purchase Orders",
    href: "/purchase-orders",
    children: [
      { label: "Create", href: "/purchase-orders/new" },
      { label: "Receive", href: "/purchase-orders/receive" },
    ],
  },
]
