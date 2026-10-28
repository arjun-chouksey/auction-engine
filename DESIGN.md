---
version: alpha
colors:
  ink: "#18222f"
  navy: "#12304a"
  action: "#1b4965"
  signal: "#a9e5ef"
  canvas: "#eef2f5"
  paper: "#ffffff"
  danger: "#a2342f"
typography:
  body:
    fontFamily: '"Avenir Next", Avenir, "Segoe UI", sans-serif'
  display:
    fontFamily: 'Georgia, "Times New Roman", serif'
  data:
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace'
rounded:
  DEFAULT: "6px"
spacing:
  field: "20px"
  section: "30px"
components:
  clockRail:
    background: "#12304a"
    signal: "#a9e5ef"
---

## Overview

The authenticated product is a procurement control room for buyers and logistics suppliers. It should feel exact, calm, and auditable—not like a trading game or generic SaaS dashboard. Its signature is the dark auction clock rail, where tabular countdown figures and absolute deadlines share one visual boundary.

## Colors

Navy communicates stable controls and deadlines; pale cyan is reserved for live timing signals. White paper surfaces sit on a cool operational canvas. Status colors must always include text, never color alone. Runtime ownership is `frontend/src/styles.css`.

## Typography

Avenir/system sans carries product workflows, labels, and tables. Georgia appears only on the sign-in thesis. Monospace is reserved for countdowns and exact, changing data.

## Layout

Desktop details use a wide evidence column and a narrower rule/audit column. Tables scroll horizontally rather than hiding columns. Below 980px, all critical content stacks in document order.

## Elevation & Depth

Static surfaces use a single hairline border and very shallow shadow. Timing emphasis comes from color contrast, not floating cards.

## Shapes

Controls and panels use a restrained 6px radius; only statuses and rank medallions use pills or circles.

## Components

The auction clock rail is the only high-contrast signature. Buttons follow primary, outline, and ghost emphasis. Loading, empty, error, success, and disabled states retain stable page structure.

## Do's and Don'ts

- Do prioritize rank, price, close time, and audit evidence.
- Do keep money tabular and exact.
- Do not add decorative charts, gradients, glass effects, or optimistic auction state.
- Do not hide data on narrow screens; preserve it through scrolling or stacking.
