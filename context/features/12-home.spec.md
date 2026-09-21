### App Home Layout

This will be on `apps/app`.
Guard the `/home` route and build its layout: side nav and top nav only, no page content.

## Requirements

### Route guard

- `/home` is only accessible when the token cookies are present, otherwise redirect to `/`

### Login page

- Remove the "Forgot password?" link from the login page

### Layout

- Design of the home page is in the prototype pdf (dashboard page)
- This feature is only the layout, side nav and top nav
- Responsive, for both desktop and mobile environments

### Side nav

- Items follow the prototype: a "Workspace" section with Dashboard, Medicines, Inventory Entries, Suppliers and Purchase Orders
- Purchase Orders is collapsible and has a sub nav with two items: Create and Receive

### Top nav

- Top left has the icon of the app, using `public/favicon.ico` as the logo; fall back to the `BrandLogo` component if it is missing
- Top right has the avatar with the first letter of the name of the user, followed by the color-mode switch button
- The token has no user name, so the avatar letter is the first letter of the email

## References

- @context/screenshots/InventraMed Prototype.pdf
- @apps/app/src/components/brand/BrandLogo.tsx
- @apps/app/src/app/home/page.tsx
