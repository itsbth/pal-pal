# Product

## Register

product

## Users

Palworld dedicated-server operators use Pal-pal to check server health, inspect online players, view player locations, and review recent metrics. Server members may use the same interface in read-only mode when the operator enables public access.

## Product Purpose

Pal-pal provides a safer, task-focused web layer over the Palworld Dedicated Server REST API. It keeps upstream API credentials server-side, records operational history locally, and separates routine observation from privileged player administration.

## Brand Personality

Practical, calm, and lightly playful. The interface should feel finished and trustworthy without becoming ornamental or visually tied to a generic infrastructure dashboard.

## Anti-references

Avoid dense enterprise-admin chrome, neon gaming dashboards, decorative fantasy styling, glass effects, and bare unstyled HTML. Server state and actions must remain easy to scan.

## Design Principles

1. Put current server health and player activity first.
2. Make read-only and administrative capabilities unmistakable.
3. Keep sensitive operational details, including player IPs and full settings, admin-only.
4. Keep dangerous actions deliberate and contextual.
5. Prefer resilient server-rendered interactions over client-side complexity.
6. Use visual character sparingly, without competing with operational data.

## Accessibility & Inclusion

Target WCAG 2.2 AA. Preserve keyboard navigation, visible focus states, sufficient contrast, meaningful status text that does not rely on color alone, reduced-motion preferences, and usable layouts from small mobile screens through desktop monitors.
