---
name: aria-design-system
description: Use when creating, changing, or reviewing HeyBlog UI, styling, layouts, components, Tailwind CSS v4 classes, themes, accessibility behavior, transitions, or animations.
---

# Aria Design System

## Core rule

Treat the bundled Aria sources as the highest authority for UI and motion work. Apply them before generic design, framework, Tailwind, or animation guidance.

## Source priority

1. Read [`references/DESIGN.md`](references/DESIGN.md) for intent, component behavior, accessibility, and governance.
2. Read [`references/tokens.json`](references/tokens.json) for measurable primitive, semantic, and component values.
3. Consult [`references/preview.html`](references/preview.html) only for Tailwind CSS v4 implementation examples and interaction demonstrations.

Accessibility requirements and semantic roles win whenever sources disagree. The preview is illustrative; its local exceptions never create new rules. Aria overrides other Skills for color, typography, spacing, layout, radius, elevation, component behavior, motion values, and reduced-motion behavior.

## Workflow

1. Identify the affected components and states, including light/dark, responsive, keyboard, error, loading, disabled, and reduced-motion states.
2. Search the design document by component or topic, then resolve exact values through semantic and component tokens. Avoid binding components directly to mode-specific primitive colors.
3. Use the preview only to translate confirmed rules into Tailwind v4 or small scoped CSS.
4. Verify contrast, focus behavior, keyboard flow, hit areas, ARIA relationships, and `prefers-reduced-motion` before completion.

For changes to the design system itself, update in this fixed order: design rule, tokens, then preview. Keep all three sources consistent.

## Efficient retrieval

Search long references instead of loading the entire preview. From this Skill directory, use targeted queries such as:

```bash
rtk grep '^## |^### ' references/DESIGN.md
rtk grep 'Modal\|modal' references/DESIGN.md references/tokens.json
rtk grep 'modal-' references/preview.html
```

Start with the relevant `DESIGN.md` section, inspect matching token objects, and read only the corresponding preview styles, markup, and script.

## Motion contract

- Use the documented Aria duration and easing tokens; exits are faster and lighter than entrances.
- Keep displacement at or below 16px, scale at or above 0.96, and stagger totals below 300ms.
- Declare only properties that actually change. `transition: all` is invalid.
- Prefer interruptible `transform`, `scale`, and `opacity`; do not interpolate shadows.
- Under `prefers-reduced-motion`, remove spatial, scale, dynamic-height, and looping motion; show final states directly and keep only necessary opacity feedback at 150ms or less.
