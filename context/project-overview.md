## Project Overview

**InventraMed** is a collaborative digital medicine inventory system. It tracks medicine expiration dates and key details (name, batch, etc.) through website-based barcode scanning, and reflects each medicine's freshness on a physical LED indicator driven by an ESP32.

No physical barcode scanner is used — medicines are registered with a generated barcode, and scanning happens directly through the website (webcam/browser-based).

---

## Problem

Manual, paper-based (or spreadsheet-based) medicine inventory tracking makes it hard to:

- Know at a glance which medicines are close to or past their expiration date
- Keep inventory data consistent across multiple people managing stock
- Get a physical, real-world signal (not just a dashboard) of stock condition

➡️ **InventraMed gives teams a shared, always-current view of medicine expiration status — both on-screen and as a physical LED signal.**

---

## Users

| Persona                | Needs                                                                |
|-------------------------|-----------------------------------------------------------------------|
| Inventory staff         | Scan/register medicines via the website, view expiration status        |
| Collaborator            | Shared, real-time visibility into inventory across the team            |
| Admin                   | Manage medicine records and inventory configuration                    |

---

## Core Features

### A) Medicine inventory management

- Register and manage medicines (name, expiration date, and other key details)
- Barcode-based lookup — barcodes are generated per medicine, not read from a physical scanner
- Scanning is performed through the website (browser-based)

### B) Expiration status

Each medicine's expiration status is computed from its expiration date and mapped to an LED indicator:

- **Green** — far from expiration
- **Yellow** — nearing expiration
- **Red** — expired / no longer safe to consume

### C) Hardware indicator (ESP32)

- ESP32 (NodeMCU) drives physical LED indicators reflecting expiration status
- Planned hardware: ESP32, LED indicators, power supply, breadboard, jumper wires

### D) Real-time sync

- The `ws` app provides real-time communication between the backend/web app and connected hardware (ESP32), keeping LED state in sync with inventory changes

---

## Tech Stack

| Category      | Choice                                    |
|----------------|--------------------------------------------|
| Language (backend) | Go 1.27                               |
| Language (frontend) | Node.js 24.20.0 / TypeScript          |
| API framework  | Gin                                        |
| Web app        | Next.js                                    |
| Database       | PostgreSQL                                 |
| ORM            | GORM                                       |
| Migrations     | Goose                                      |
| Monorepo       | Nx (`@nx-go/nx-go` for Go apps, `@nx/next` for the web app) |
| Hardware       | ESP32 (NodeMCU) + LED indicators           |

---

## Monorepo Structure

InventraMed is an Nx monorepo with three apps under the `apps/` directory.

```
apps/
├── api    # Go REST API (Gin) — inventory/medicine endpoints, backed by PostgreSQL via GORM
├── ws     # Go WebSocket service — real-time sync between backend and ESP32 LED indicators
└── app    # Next.js web app — inventory dashboard and browser-based barcode scanning
```

### `apps/api`
Go REST API built with **Gin**. Owns medicine/inventory data via PostgreSQL (GORM), and exposes endpoints consumed by `apps/app`.

### `apps/ws`
Go WebSocket service. Bridges real-time state (expiration status changes) between the backend and the ESP32 hardware driving the LED indicators.

### `apps/app`
Next.js web app. Provides the inventory dashboard and browser-based barcode scanning UI.

---

## Data Model (Overview)

*To be defined as the schema is implemented (via Goose migrations).*

- **Medicine** — name, expiration date, barcode, and derived expiration status (green/yellow/red)

---

## Status

- 🚧 Early scaffold — Nx monorepo, Go apps (`api`, `ws`), and Next.js app (`app`) are initialized; core features not yet implemented
