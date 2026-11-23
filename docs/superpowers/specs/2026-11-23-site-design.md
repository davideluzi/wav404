# wav404 presentation site (GitHub Pages)

Date: 2026-11-23. Status: awaiting approval.

## Why

SP-404SX owners look for a way to load samples onto the card because Roland's
*SP-404SX Wave Converter* is discontinued. It only runs on Intel Macs through
Rosetta and on old Windows versions. The page must tell them in a few seconds
that wav404 is that tool, and get them to the download without friction.

## Audience and goal

- Audience: SP-404SX owners who want to download the app. They are not developers.
- Success: they reach the right download for their OS, open the app on the first
  try (including the macOS "Open Anyway" step), and back up their card first.

## Non-goals

- No documentation site, blog or changelog. GitHub releases cover the changelog.
- No Linux binary (none is built). Linux users get a link to "build from source" in the README.
- No analytics or cookies.

## Anti-goals

- Any Roland logo, product photo, artwork or look. SP-404SX is only named
  for compatibility, together with the "not affiliated" disclaimer.
- Real card samples in screenshots or in the repo.

## Approaches

- **A. One static HTML page, like Feedy (chosen by the user).** `site/index.html`
  with inline CSS, plus a Pages workflow. It gives full control and has no dependencies.
- B. README rendered by Jekyll with a GitHub theme. Rejected: it looks like documentation,
  and download buttons and a screenshot gallery work poorly.
- C. A site generator (Hugo, Astro). Rejected: too much for one page.

## Design

**Structure (top to bottom)**
1. Hero: icon, "wav404", one line ("Load samples onto your SP-404SX card on
   macOS, Windows and Linux"), and the main screenshot. A short line says
   it replaces the discontinued Wave Converter that needs an Intel Mac / Rosetta.
2. Download buttons:
   - macOS: `https://github.com/davideluzi/wav404/releases/latest/download/wav404-macos-universal.dmg`
   - Windows: `…/releases/latest/download/wav404-windows-amd64.zip`

   Under the buttons: the requirements (macOS 13+ universal, Windows 10+ 64-bit),
   and links to all releases and to the source / command-line tool.
3. Features: 3–4 screenshots with short captions. The captions cover banks and
   pads, the waveform trim, the pad settings (volume, loop, gate, reverse, lo-fi,
   BPM), and the import with drop highlight and progress.
4. First launch on macOS: the "Open Anyway" steps (System Settings → Privacy &
   Security), plus the right-click → Open alternative on macOS 13–14.
5. "Back up your card first", then the footer: MIT license, GitHub link,
   "Not affiliated with Roland" disclaimer.

**Look:** follows the app's identity: graphite background, amber accent
(#ffb020), and a 7-segment style for the hero name or the display only, with
system fonts for the text. Light and dark mode: the page is dark by design,
like the hardware; a light variant is optional. It must work at phone width.

**Screenshots**
- They are rendered from the real UI by a gated test, `WAV404_SITE_SHOTS=<dir>`,
  on the synthetic fixture card. They are written to `assets/screenshots/*.png`
  and committed, so the site rebuilds without Go.
- States: main view (bank selected, pad with waveform), a playing pad, a drag
  highlight on a pad, and the import progress dialog.
- Sizes are fixed, at 2× scale for sharp images.

**Publishing**
- `.github/workflows/pages.yml` mirrors Feedy's. It runs on a push to `main`
  that touches `site/**`, `assets/screenshots/**` or the workflow, copies the
  screenshots into the site, and deploys with `actions/deploy-pages`.
- One-time manual step for the user: Settings → Pages → Source: GitHub Actions.
- The README links to the site.

## Left to the builder

- Exact copy wording (English, like the app and the README).
- The icon and favicon come from `cmd/wav404/Icon.png`.
- A rendered preview of the page is shown to the user before publishing.
