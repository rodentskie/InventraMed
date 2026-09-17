# InventraMed

Development of a Collaborative Digital Medicine Inventory System

## Overview

InventraMed is a system focused on monitoring the expiration dates of medicines, along with other key details such as the medicine's name. The core function of the system is an inventory that lights up based on readings from a scanner. Since our panelist recommended using a barcode generator to allow medicine information to be scanned, no physical barcode scanner is used — scanning is instead handled directly through the website.

Based on how close a medicine is to its expiration date, the system lights up a corresponding LED indicator:

- **Green LED** — the medicine is far from its expiration date
- **Yellow LED** — the medicine is nearing its expiration date
- **Red LED** — the medicine is expired or no longer safe to consume

## Hardware Components (Planned)

- ESP32 (NodeMCU)
- LED indicators
- Power supply
- Breadboard
- Wires / Jumper wires
