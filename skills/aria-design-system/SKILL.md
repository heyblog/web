---
name: aria-design-system
description: Use when creating, changing, or reviewing HeyBlog UI, styling, layouts, components, Tailwind CSS v4 classes, themes, accessibility, or motion. Do not use for logic-only, API, or content-data changes with no UI effect.
---

# Aria Design System

## Core rule

Treat the bundled Aria sources as the highest authority for UI and motion work. Apply them before generic design, framework, Tailwind, or animation guidance.

## Source priority

1. Locate the relevant component or topic in [`references/DESIGN.md`](references/DESIGN.md). Read
   only that section plus any global accessibility or motion section required by the task.
2. Read the matching semantic or component subtree in
   [`references/tokens.json`](references/tokens.json) for measurable values.
3. Consult only the corresponding part of
   [`references/preview.html`](references/preview.html) when an implementation example or interaction
   demonstration is needed.

Accessibility requirements and semantic roles win whenever sources disagree. The preview is illustrative; its local exceptions never create new rules. Aria overrides other Skills for color, typography, spacing, layout, radius, elevation, component behavior, motion values, and reduced-motion behavior.

## Workflow

1. Identify the affected components and states, including light/dark, responsive, keyboard, error, loading, disabled, and reduced-motion states.
2. Search the design document by component or topic, then resolve exact values through semantic and component tokens. Avoid binding components directly to mode-specific primitive colors.
3. Use the preview only to translate confirmed rules into Tailwind v4 or small scoped CSS.
4. Verify contrast, focus behavior, keyboard flow, hit areas, ARIA relationships, and `prefers-reduced-motion` before completion.
5. Report the affected states, the semantic or component tokens used, and the validation evidence.

For changes to the design system itself, update in this fixed order: design rule, tokens, then preview. Keep all three sources consistent.

## Efficient retrieval

Search long references instead of loading the entire preview. From this Skill directory, use targeted queries such as:

```bash
rg -n '^## |^### ' references/DESIGN.md
rg -ni 'modal' references/DESIGN.md references/tokens.json
rg -n 'modal-' references/preview.html
```

Start with the relevant `DESIGN.md` section, inspect matching token objects, and read only the corresponding preview styles, markup, and script.

## Motion contract

- Resolve duration, easing, distance, scale, and stagger values from the current Aria motion tokens;
  exits remain faster and lighter than entrances.
- Declare only properties that actually change. `transition: all` is invalid.
- Prefer interruptible `transform`, `scale`, and `opacity`; do not interpolate shadows.
- Follow the documented `prefers-reduced-motion` behavior and show final states directly when motion
  is removed.
