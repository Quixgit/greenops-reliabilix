# ADR-0036: Multi-language platform

Status: accepted · 0.1.0-alpha.34

## Decision
1. **The server speaks codes, the client speaks languages.** API problems carry a stable `code` (the English title as a
   slug) next to `title` and `detail`; `title` is translated server-side from `Accept-Language` when a translation exists,
   and `Vary: Accept-Language` is set. Domain data (service names, regions, IDs) is never translated.
2. **Server texts** (problem titles, e-mail and report wording) live in `internal/platform/i18n/locales/<tag>.json`.
   `i18n.T(lang, key, name, value, ...)` falls back to English and then to the key (a visible, safe failure).
3. **Frontend catalogs** are flat JSON files in `frontend/i18n/messages/<tag>.json`; English is the source and the
   fallback. `useT()` gives `t(key, params, fallback)`; `{name}` placeholders; plural forms are separate keys
   (`key.one|few|many|other`) chosen with `Intl.PluralRules`. Numbers, money and dates follow the language through
   `Intl` (`setFormatLocale`).
4. **Language selection without URL prefixes**: the cookie `rlx_locale` (set by `POST /api/locale`, only supported tags
   accepted), else the browser's `Accept-Language`, else the default. Routes, sign-in and the API proxy stay unchanged.
   `<html lang>` follows. A URL-prefix scheme (`/ru/...`) is a later option if SEO or sharing by language is needed.
5. **Offered languages are platform settings** (`i18n.enabled_locales`, `i18n.default_locale`), editable in the console.
   A language can only be enabled if the build has its catalog.
6. **Parity is a build failure**: tests require every language to have exactly the keys of English with the same
   placeholders (backend and frontend).

## How to add a language
Add `locales/<tag>.json` (backend) and `messages/<tag>.json` (frontend) with every key of English, add the tag to
`i18n.Supported`, `LOCALES` and the enums of the two settings, and the loader line in `i18n/server.ts`. Tests tell what is
missing.

## State and consequences
- Done now: the mechanism end to end, the shell of both contours (menus, header, language switcher), the Carbon & Energy
  page headings, server problem titles. **Not translated yet:** most page bodies, forms, validation details and e-mail
  templates; they show English until their keys are added (the fallback is English, never a broken string).
- Rule for new code: a user-visible string gets a key in both catalogs (`CLAUDE.md`).
- A per-user stored preference (instead of the cookie) and a per-recipient language for e-mail and reports are the next
  step; they need a user profile setting.
