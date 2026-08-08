---
name: agentmeter
description: A darkroom instrument for reading how much agent headroom is left, glanced at from two metres.
colors:
  ground: "#0b0806"
  chassis: "#14100c"
  edge: "#2a2018"
  paper: "#ece2d0"
  paper-dim: "#b9ab94"
  ink: "#f2ebdd"
  muted: "#9a8b76"
  safelight: "#ff9a3c"
  safelight-dim: "#6b3d16"
  exposed: "#0a0705"
  caution-ink: "#e0c07a"
  caution-edge: "#5c4a1e"
  caution-ground: "#1b1409"
typography:
  density:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "clamp(2.1rem, 4vw, 3rem)"
    fontWeight: 600
    letterSpacing: "-.04em"
    fontFeature: "tabular-nums"
  readout:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "clamp(1.15rem, 2.4vw, 1.6rem)"
    fontWeight: 600
    letterSpacing: "-.02em"
    fontFeature: "tabular-nums"
  section-label:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "0.72rem"
    fontWeight: 600
    letterSpacing: "0.2em"
  rebate-label:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "0.68rem"
    fontWeight: 600
    letterSpacing: "0.22em"
  control-label:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "0.66rem"
    fontWeight: 400
    letterSpacing: "0.14em"
  figure:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "0.84rem"
    fontWeight: 400
    fontFeature: "tabular-nums"
  body:
    fontFamily: "IBM Plex Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.8rem"
    fontWeight: 400
    lineHeight: 1.5
rounded:
  step: "1px"
  control: "2px"
  sheet: "3px"
spacing:
  hairline: "2px"
  xs: "4px"
  sm: "9px"
  md: "14px"
  lg: "18px"
  xl: "22px"
  gutter: "44px"
  foot: "72px"
components:
  sheet:
    backgroundColor: "{colors.chassis}"
    textColor: "{colors.ink}"
    rounded: "{rounded.sheet}"
    padding: "20px 22px 22px"
  sheet-estimated:
    backgroundColor: "{colors.chassis}"
    textColor: "{colors.paper-dim}"
    rounded: "{rounded.sheet}"
  wedge:
    backgroundColor: "#070504"
    rounded: "{rounded.control}"
    padding: "3px"
    height: "52px"
  wedge-step-exposed:
    backgroundColor: "{colors.exposed}"
    rounded: "{rounded.step}"
  rebate:
    backgroundColor: "{colors.chassis}"
    textColor: "{colors.safelight}"
    typography: "{typography.rebate-label}"
    padding: "9px 22px"
  range-button:
    backgroundColor: "transparent"
    textColor: "{colors.muted}"
    typography: "{typography.control-label}"
    rounded: "{rounded.control}"
    padding: "5px 10px"
  range-button-active:
    textColor: "{colors.safelight}"
  lamp:
    backgroundColor: "#0d0906"
    textColor: "{colors.muted}"
    rounded: "{rounded.control}"
    padding: "5px 11px 5px 8px"
  lamp-on:
    textColor: "{colors.safelight}"
  origin-chip:
    textColor: "{colors.paper-dim}"
    rounded: "{rounded.control}"
    padding: "5px 11px"
  origin-chip-stale:
    backgroundColor: "{colors.caution-ground}"
    textColor: "{colors.caution-ink}"
    rounded: "{rounded.control}"
  notice:
    backgroundColor: "{colors.caution-ground}"
    textColor: "{colors.caution-ink}"
    rounded: "{rounded.sheet}"
    padding: "12px 15px"
---

# Design System: agentmeter

## Overview

**Creative North Star: "The Darkroom Under Safelight"**

The surface is a print room at working temperature: a warm near-black ground, one amber
safelight, and photographic paper as the only bright material on screen. Nothing here is a
dev dashboard. There is no wordmark at display size, no stat-card grid, no line chart. The
limits *are* the page: they open the first viewport at full width and everything else —
history, cost, per-agent tables — sits below the fold as reference.

The thesis is that headroom reads as light. A filled progress bar tells you how much you
have spent; a sheet of paper tells you how much is left to expose. The meter is therefore a
calibrated step tablet of ten patches whose tone is fixed by position and never by the
value, blackened from the left as usage consumes it. The reading is which patch is the last
light one, and that is legible from two metres with both hands busy — the confirmed use
scene is a second monitor, running for hours, self-refreshing.

Density is instrument density, not marketing density. Type is small, letterspaced, and
almost entirely monospaced; the two-digit percentage is the only thing allowed to be large.
Grain lies over the whole surface at low opacity so the dark ground reads as material rather
than as an empty void. Depth is drawn with hairline edges and tonal steps between ground,
chassis and paper — not with shadow.

**Key Characteristics:**
- Warm near-black ground; paper is the only bright field.
- One accent — amber safelight — and it only ever means live, active, or at the limit.
- Ten-patch step tablet with fixed graduation; exposure eats it from the left.
- Mono for every measured value, with tabular numerals; sans for prose only.
- Full-surface film grain; hairline borders; radii of 1–3px.
- One authored motion moment, on first mount only.

## Colors

A warm monochrome darkroom — every neutral sits on the same amber-adjacent hue family — cut
by exactly one saturated accent.

### Primary
- **Safelight Amber** (`{colors.safelight}`): the only saturated color in the system. It
  carries the rebate markings, the live lamp when lit, the reset time on each sheet, the
  active range button, focus rings, the wet edge of the patch mid-exposure, and the frame of
  a critical sheet. Nothing decorative ever takes it.
- **Safelight Dim** (`{colors.safelight-dim}`): the lamp at rest and the border of any
  control that is on but not shouting. It is the accent's off-state, not a second accent.

### Secondary
- **Caution Amber-Gold** (`{colors.caution-ink}` on `{colors.caution-ground}` with
  `{colors.caution-edge}`): reserved for the system admitting a gap in its own data — the
  unpriced-model notice, a lamp error, a stale or unavailable origin chip. It is deliberately
  desaturated relative to the safelight so it never competes with a real reading.

### Neutral
- **Darkroom Ground** (`{colors.ground}`): the page. Warm black, never blue-black.
- **Chassis** (`{colors.chassis}`): every sheet, readout cell and panel; one step up from the
  ground, which is the entire elevation vocabulary.
- **Rebate Edge** (`{colors.edge}`): all hairline borders, table rules and grid seams.
- **Photographic Paper** (`{colors.paper}`): the light end of the wedge graduation, the
  history strip's unexposed cells, and button hover text.
- **Paper Dim** (`{colors.paper-dim}`): table figures, chip text, and every numeral on an
  estimated sheet — paper that has already taken some exposure.
- **Ink** (`{colors.ink}`): default body and headline text, and the live density numeral.
- **Muted** (`{colors.muted}`): labels, section headings, legends, idle reset text, controls
  at rest. Most type on this page is this color.
- **Exposed** (`{colors.exposed}`): a consumed patch. Slightly darker than the ground so a
  fully exposed wedge still reads as a thing sitting on the page.

### Named Rules
**The One Safelight Rule.** There is one accent hue. If a new element wants a color, it gets
muted, paper-dim, or nothing. Amber is spent only on *live*, *active*, *focused*, or *at the
limit*.

**The Fixed Graduation Rule.** The wedge's paper tones (`hsl(34 22% 52%)` at the exposed end
to `hsl(34 22% 94%)` at the fresh end) are a function of patch position, never of the value.
A scale that moves with its reading is not a scale.

**The Provenance Rule.** Alarm color belongs to measured readings only. A derived estimate
never turns amber and never turns white — it goes dashed, drops to paper-dim, and labels
itself `uncalibrated`. This is a product constraint from PRODUCT.md before it is a visual one.

## Typography

**Body Font:** IBM Plex Sans (with `ui-sans-serif`, `system-ui`)
**Measurement Font:** IBM Plex Mono (with `ui-monospace`, `monospace`)

Both are self-hosted as woff2 and embedded in the binary — a dashboard about private usage
never calls a font CDN. Four files ship: Sans 400/600 and Mono 400/600. There are no other
weights, and no italics.

**Character:** engineered and unglamorous. Plex Mono does the instrument work — it is the
face of a densitometer readout — while Plex Sans appears only where actual sentences occur.
The register is a lab panel someone reads at speed, not a product page.

### Hierarchy
- **Density** (`{typography.density}`): the two-digit percentage beside each wedge. The only
  large type on the page, negatively tracked and right-aligned against a `3.6ch` minimum so
  digits never shift the wedge. Its trailing `%` drops to 0.42em in muted.
- **Readout** (`{typography.readout}`): the four totals in the readout row.
- **Section Label** (`{typography.section-label}`, uppercase): every `h2`. Headings are
  labels here, not headlines — they identify a panel and then get out of the way.
- **Rebate Label** (`{typography.rebate-label}`, uppercase): the top strip's markings, the
  widest tracking in the system.
- **Control Label** (`{typography.control-label}`, uppercase): range buttons (0.14em), lamps
  and chips (0.16–0.18em), strip legend (0.62rem/0.12em).
- **Figure** (`{typography.figure}`): table cells and inline measured values inside prose.
- **Body** (`{typography.body}`): notices, wedge notes, status text. Errors cap at `44ch`.

### Named Rules
**The Mono-Is-Measurement Rule.** Every percentage, token count, cost, reset time, table
figure and label is set in Plex Mono with `tabular-nums`. Sans is for sentences only. When
prose contains a measured value, that segment switches to mono mid-sentence so the figure
lines up with the instrument above it.

**The Label-Not-Headline Rule.** No type between 0.84rem and 2.1rem exists. A surface either
whispers at label scale or states a reading at density scale; there is no middle voice.

## Layout

A single centered column, `min(1240px, 100% - 44px)`, with `72px` of foot clearance. The
rebate strip is full-bleed above it.

Limit sheets are the first and largest thing: an auto-fit grid with a `430px` minimum track,
so two sheets sit side by side on a wide monitor and one fills the width otherwise. Its
control bar spans the full grid (`grid-column: 1 / -1`). Below the fold, history sits in one
sheet, the four totals in a four-column readout row seamed with 1px gaps that show the border
color through, and the two log tables in a `1fr 1.35fr` pair.

Spacing rhythm is tight and odd-numbered by intent: `2px` between patches and cells, `9/10px`
inside controls, `14/16/18px` between related blocks, `22px` inside a sheet and between
wedges. Sheets stack at `18px`.

One breakpoint, at `900px`: the gutter narrows to `28px`, limits, logs collapse to one
column, readouts fold to 2×2, the sprocket perforations are dropped, and the wedge shortens
from `52px` to `42px`. Nothing else changes — the instrument is the same instrument on a
laptop.

### Named Rules
**The Limits-Are-The-Page Rule.** The first viewport is the rebate strip and the limit
sheets. Nothing — no wordmark block, no summary cards, no chart — may be inserted above them.

## Elevation & Depth

No shadows are used for elevation. Depth is tonal and hairline: ground → chassis → paper, one
step apart, separated by 1px borders in the edge color. Every panel sits flat on the page.

Shadow exists only as light, never as lift. Three of the four shadow declarations are inset
1px rims that give a patch its own boundary; the two outer glows are safelight bloom on a lit
lamp and on a critical wedge frame. A shadow in this system means something is emitting.

### Shadow Vocabulary
- **Patch rim** (`inset 0 0 0 1px rgb(10 7 5 / .35)`, and `rgb(255 231 199 / .07)` on an
  exposed patch, `rgb(255 231 199 / .08)` on a history cell): separates adjacent tones so a
  ten-step ramp stays countable.
- **Wet edge** (`inset 0 0 0 1px rgb(255 154 60 / .55), 0 0 11px -2px rgb(255 154 60 / .7)`):
  the single patch currently mid-exposure.
- **Critical frame** (`0 0 0 1px rgb(255 154 60 / .5), 0 0 26px -6px rgb(255 154 60 / .55)`):
  a live reading at or above 90.
- **Lamp bloom** (`0 0 9px 1px rgb(255 154 60 / .85)`): the lit safelight dot.

### Named Rules
**The Emission Rule.** Shadows are glow, not drop. If an element is not emitting safelight,
it gets a border and no shadow.

## Shapes

Radii are almost absent and strictly tiered: `3px` for sheets, notices and the readout block;
`2px` for controls, chips and the wedge frame; `1px` for individual patches and history cells.
Nothing is pill-shaped except the 8px lamp dot, which is a circle because it is a bulb.

Borders are always 1px and always the edge color, with three deliberate exceptions: the
safelight-dim border of an engaged control, the safelight border of a critical wedge, and the
caution border of a notice. Dashed 1px is a semantic, not a style: it marks an instrument that
is not measuring — the loading status block, an estimated sheet, and that sheet's wedge frame.

The recurring silhouette is the strip: a horizontal run of small equal cells with 2px gaps —
the wedge, the history test strip, the readout row, and the perforations all repeat it.

## Components

### Range Buttons (Navigation)
Ghost by default, and the only navigation on the page.
- **Shape:** near-square (2px), 1px transparent border reserving the active state's space.
- **Default:** transparent on chassis, muted uppercase mono at 0.66rem/0.14em.
- **Hover:** text lifts to paper. No background change, ever.
- **Active:** safelight text on a safelight-dim border.
- **Focus:** `2px solid` safelight ring, offset `2px`. Identical across every focusable
  element in the system.

### Sheets (Cards)
The primary container; a sheet of paper in a tray.
- **Corner Style:** 3px.
- **Background:** chassis, on the darkroom ground.
- **Border:** 1px edge. Dashed when the sheet's data is estimated.
- **Shadow:** none (see Elevation).
- **Padding:** `20px 22px 22px`; head row is a baseline-aligned flex with 16px gap.

### Lamp (Toggle)
The live switch, styled as a safelight rather than as a checkbox. The native input is
visually hidden and the label carries every state via `:has()`.
- **Off:** edge border, muted text, dot in safelight-dim.
- **On:** safelight-dim border, safelight text, dot in full safelight with bloom.
- **Busy:** 50% opacity, `cursor: progress`.

### Origin Chips
A one-word statement of where a reading came from.
- **Default (estimated):** edge border, paper-dim, uppercase mono 0.74rem/0.16em, weight 600.
- **Live:** safelight-dim border with a softened amber text.
- **Stale / unavailable:** the caution palette — gold on dark gold with a gold border.

### Tables
- Right-aligned, first column left. 1px top rules only; no verticals, no zebra.
- Headers in mono 0.62rem/0.16em uppercase muted at weight 400.
- Every non-first cell is mono with tabular numerals in paper-dim; the label column stays in
  the default ink so the figures read as a block.

### Step Wedge (Signature Component)
The instrument the whole world exists to hold. Ten patches in a 52px frame with 2px gaps.
Each patch's tone comes from its index (`hsl(34 22% L%)`, L from 52 to 94) and is fixed.
Usage exposes patches from the left; the single patch straddling the value gets a hard-stop
linear gradient at the exact crossing percentage plus the wet-edge glow. Ten was chosen over
twenty because twenty leaves too little tone between neighbours to read at distance.

Beside it sits the density numeral: ink when live and normal, safelight at 75+, white on a
safelight-framed wedge at 90+, and paper-dim whenever the reading is estimated.

### Test Strip (Signature Component)
Daily history as one exposure per day, read by density, not as a bar chart — a bar chart at
88px cannot be read across a room. Lightness runs `90% - (used/max) * 72` on the same paper
hue, stopping short of the ground so the busiest day reads as the darkest exposure rather
than as a hole. Idle days are filled in rather than omitted, so the strip does not misreport
elapsed time. A two-ended legend below states the direction of the scale.

### Motion
One authored moment: the `develop` keyframe (opacity 0→1, `brightness(.35)`→none) over
`.9s cubic-bezier(.16, 1, .3, 1)`, playing once on first mount and gated off by the
`is-settled` class thereafter. The 60-second limit poll and the 180-second history poll
replace values silently. The animation is removed entirely under
`prefers-reduced-motion: reduce`.

**The Print-Comes-Up-Once Rule.** Motion belongs to arriving at the page. A refresh is not an
arrival, and nothing on this surface may animate on a poll.

## Do's and Don'ts

### Do:
- **Do** put every measured value in IBM Plex Mono with `font-variant-numeric: tabular-nums`,
  including values embedded mid-sentence.
- **Do** express any new meter as a fixed graduation that gets consumed, not as a fill that
  grows.
- **Do** reserve safelight amber for live, active, focused, and at-limit states.
- **Do** mark an instrument that is not measuring with a dashed 1px border and a word — never
  with color alone.
- **Do** keep new panels flat: chassis fill, 1px edge border, 3px radius, no shadow.
- **Do** give every focusable element the same `2px solid` safelight ring at `2px` offset.
- **Do** design for a two-metre glance first: if a new element cannot be read from across the
  room, it belongs below the fold.

### Don't:
- **Don't** introduce a second accent hue. Caution gold is for the system's own data gaps and
  is not available as decoration.
- **Don't** apply warn or critical styling to a derived estimate; a thin baseline hitting
  100% is an artefact, and coloring it is the page telling a lie.
- **Don't** animate anything on the refresh poll.
- **Don't** use drop shadows for elevation; depth is tonal, and glow means emission.
- **Don't** add a display-scale wordmark or a masthead block above the limits.
- **Don't** add a line or bar chart to this surface; history is read as density.
- **Don't** fetch a font, script or stylesheet from a CDN — the three files are served
  verbatim from the Go binary's embedded filesystem, with no build step and no framework.
- **Don't** exceed the four shipped font files (Sans 400/600, Mono 400/600), and don't use
  italics.

---

*Recorded from the shipped build of `web/style.css`, `web/index.html` and `web/app.js`.
Review status, stated honestly: the finish review returned disposition "fix" twice. Eight
material findings were applied and scored (seven resolved, one partial); the partial and
three regressions were then fixed, but the second verdict pass never completed — the reviewer
terminated on a session limit. The final batch is unverified by the reviewer, and this build
is not review-passed.*
