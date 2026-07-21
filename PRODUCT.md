# PRODUCT.md

## Register

product

## Users & Purpose

Developers exploring code structure and symbol relationships in a local tree. They open a directory, click through definitions and usages, and jump between files via the same `provider:path::symbol` references the CLI uses.

Primary task on any screen: read source with every resolved symbol clickable, and land on the definition (anchor) in the target file.

## Brand Personality

precise, terminal-native, amber-focused

## UI stack (serve SPA)

Tailwind CSS 4 + daisyUI 5. Prefer daisyUI components and semantic colors (`primary`, `base-*`, …). Custom theme name: `refactree` (dark amber). Force-graph canvas node colors may stay fixed for readability.

## Anti-references

SaaS cream dashboards, glassmorphism, gradient text, hero metrics, card grids, tiny uppercase eyebrows on every section

## Accessibility

WCAG AA contrast on body text and links; keyboard-focusable symbol links; respect `prefers-reduced-motion`
