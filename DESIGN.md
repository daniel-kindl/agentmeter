---
name: agentmeter
description: A plain dark instrument for reading how much agent headroom is left, glanced at from two metres.
colors:
  bg: "#0d0f12"
  surface: "#14171c"
  surface-2: "#1b1f26"
  border: "#262b33"
  border-strong: "#333a45"
  text: "#e8ebef"
  text-dim: "#a3acb9"
  text-mute: "#78828f"
  accent: "#4d9fff"
  ok: "#3ecf8e"
  warn: "#f5a524"
  crit: "#f0616d"
typography:
  percentage:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "clamp(2.25rem, 5vw, 3.25rem)"
    fontWeight: 600
    letterSpacing: "-0.03em"
    lineHeight: 1
    fontFeature: "tabular-nums"
  readout:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "1.375rem"
    fontWeight: 600
    fontFeature: "tabular-nums"
  heading:
    fontFamily: "IBM Plex Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 600
  body:
    fontFamily: "IBM Plex Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "IBM Plex Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 400
  caption:
    fontFamily: "IBM Plex Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 400
  figure:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    fontFeature: "tabular-nums"
rounded:
  control: "4px"
  card: "6px"
spacing:
  hairline: "2px"
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "20px"
  gutter: "48px"
  foot: "64px"
components:
  card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    rounded: "{rounded.card}"
    padding: "20px"
  topbar:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    padding: "12px 24px"
  meter:
    backgroundColor: "{colors.surface-2}"
    rounded: "{rounded.control}"
    height: "12px"
  meter-fill:
    backgroundColor: "{colors.ok}"
    rounded: "{rounded.control}"
  meter-fill-warn:
    backgroundColor: "{colors.warn}"
  meter-fill-critical:
    backgroundColor: "{colors.crit}"
  meter-fill-derived:
    backgroundColor: "{colors.text-mute}"
  chart-bar:
    backgroundColor: "{colors.text-dim}"
    rounded: "2px 2px 0 0"
  chart-bar-idle:
    backgroundColor: "{colors.border-strong}"
  range-button:
    backgroundColor: "transparent"
    textColor: "{colors.text-mute}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "4px 10px"
  range-button-active:
    backgroundColor: "{colors.surface-2}"
    textColor: "{colors.text}"
  lamp:
    backgroundColor: "{colors.surface-2}"
    textColor: "{colors.text-dim}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "5px 10px 5px 9px"
  lamp-on:
    textColor: "{colors.text}"
  origin-chip:
    textColor: "{colors.text-mute}"
    typography: "{typography.caption}"
    rounded: "{rounded.control}"
    padding: "3px 8px"
  origin-chip-live:
    textColor: "{colors.accent}"
  origin-chip-stale:
    textColor: "{colors.warn}"
  notice:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text-dim}"
    rounded: "{rounded.control}"
    padding: "11px 14px"
---

# Design System: agentmeter

## Overview

**Creative North Star: "Read it from two metres."**

This surface has one job and a fixed viewing distance. It runs on a second monitor in a room
lit by other screens, is left open for hours, and is glanced at while both hands are busy
elsewhere. It must answer *how much headroom is left, and when does it come back* without
being touched, clicked, or leaned toward.

Every decision here is downstream of that distance. The interface is deliberately plain:
neutral dark surfaces, one step of elevation, hairlines instead of shadow, sentence-case
labels at readable sizes, and exactly one large element per meter. Nothing is styled to be
noticed; things are styled to be read.

This replaced an earlier metaphor-driven system ("a darkroom under safelight") that encoded
the limit as a ten-patch tonal step wedge and the history as cell lightness. Both were
internally coherent and both failed the distance test — a value you have to decode is not a
value you can glance at.

**Key characteristics:**
- Neutral near-black ground; a single surface step; hairline borders; no shadows.
- One accent for interaction, three status colours for state, and no decorative colour at all.
- The limits open the page at full width; history, cost and tables sit below as reference.
- Meters read by length. History reads by height. Both are the channels people judge most
  accurately at a distance.
- Mono with tabular numerals for every number; sans in sentence case for everything else.
- One motion rule on the whole page.

## Colors

A single neutral ramp, one interactive accent, and a three-step status scale. Every text
colour was measured against its rendered backdrop in the browser; the lowest ratio on the page
is 4.61:1, above the 4.5:1 AA floor for normal text.

### Primary
- **Accent Blue** (`{colors.accent}`, 7.0:1): interaction and only interaction — the live
  switch when lit, the live origin chip, every focus ring. It never marks state.

### Status
- **OK Green** (`{colors.ok}`, 9.4:1): a live meter below 75%.
- **Warn Amber** (`{colors.warn}`, 9.5:1): a live meter from 75% to 89%, the unpriced-model
  notice rule, a stale or unavailable origin chip, and the live-switch error.
- **Crit Red** (`{colors.crit}`, 6.0:1): a live meter at 90% or above, and the percentage
  beside it.

### Neutral
- **Ground** (`{colors.bg}`): the page.
- **Surface** (`{colors.surface}`): every card, the top bar, each totals cell. One step up from
  the ground, and the entire elevation vocabulary.
- **Surface 2** (`{colors.surface-2}`): the meter track and the active range button.
- **Border** (`{colors.border}`): all hairlines, table rules and grid seams.
- **Border Strong** (`{colors.border-strong}`): the active button's edge and an idle chart bar.
- **Text** (`{colors.text}`, 16.0:1): headings, values, the percentage.
- **Text Dim** (`{colors.text-dim}`, 8.2:1): labels, prose, table figures, chart bars.
- **Text Mute** (`{colors.text-mute}`, 4.61:1): captions, axis labels, controls at rest, and a
  derived meter's fill. Nothing essential is set below this.

### Working values
- **Meter Tick** (`--meter-tick`, `rgb(0 0 0 / .45)`): the quarter notches on the meter. An
  alpha rather than a colour, because it has to read on the empty track and on all four fill
  colours alike.

### Named rules

**The Accent Is Not A State.** Blue means *you can operate this*. If a new element wants to
say something is wrong, nearly full, or fine, it uses the status scale. If it wants attention
for its own sake, it gets a neutral.

**The Provenance Rule.** Status colour belongs to measured readings only. A derived estimate
is measured against the busiest window in local history, so it reaches 100% the moment the
current window is the busiest one recorded — an artefact of a thin baseline, not a limit being
reached. A derived meter therefore stays `{colors.text-mute}` at every value and labels itself
`uncalibrated`. This is a product constraint from PRODUCT.md before it is a visual one, and it
is enforced in `meterState()` in `web/app.js`.

**No Shadows.** Depth is the ground/surface step plus a hairline. Shadow blurs at the viewing
distance this page is designed for and buys nothing at any other.

## Typography

**Body:** IBM Plex Sans (with `ui-sans-serif`, `system-ui`)
**Numbers:** IBM Plex Mono (with `ui-monospace`, `monospace`)

Both are self-hosted woff2 embedded in the binary — a dashboard about private usage never
calls a font CDN. Four files ship: Sans 400/600 and Mono 400/600. No other weights, no italics.

Every number on the page is mono with `font-variant-numeric: tabular-nums`, so a figure that
changes on a poll does not reflow the row it sits in. Everything else is sans in sentence case
at normal tracking. There is no uppercase-plus-letterspacing treatment anywhere; the previous
system set nearly all of its labels that way, which is the least legible configuration
available for small text.

### Hierarchy
| Role | Token | Size |
|---|---|---|
| Meter percentage | `{typography.percentage}` | clamp(2.25rem, 5vw, 3.25rem) / 600 |
| Totals readout | `{typography.readout}` | 1.375rem / 600 |
| Card heading | `{typography.heading}` | 0.875rem / 600 |
| Body and messages | `{typography.body}` | 0.9375rem / 400 |
| Window label, reset, note, controls | `{typography.label}` | 0.8125rem / 400 |
| Captions, axis, chips, table headers | `{typography.caption}` | 0.75rem / 400 |
| Table figures | `{typography.figure}` | 0.875rem / 400, tabular |

### Named rules

**One Large Thing.** The meter percentage is the only element allowed above 1.5rem. It is what
resolves first from across the room; if a second element competed, neither would.

## Layout

`main` is `min(1180px, 100% - 48px)`, centred, with the top bar full-bleed above it.

Reading order is fixed by the job: limits, totals, the unpriced-model notice, daily usage, then
the per-agent and per-model tables. Limits are a `repeat(auto-fit, minmax(360px, 1fr))` grid so
two agents sit side by side on a wide screen and stack below 900px. Cards are top-aligned, not
stretched — matching a two-row table to a nine-row one only buys empty space.

### Named rules

**Every Track Is `minmax(0, …)`.** A grid track sized `auto` refuses to shrink below its
content, so one long model name is enough to push the whole document wider than the viewport.
Tables scroll inside their own card instead.

**Breakpoints:** 900px collapses every multi-column grid to one column; 480px stacks the
totals row and tightens the top bar.

## Elevation & depth

Two surfaces and a hairline. `{colors.bg}` is the page, `{colors.surface}` is anything sitting
on it, `{colors.border}` separates things that touch. There is no third level, no shadow
vocabulary, and no blur.

## Shapes

`{rounded.control}` on controls, chips, meters and notices; `{rounded.card}` on cards. Chart
bars are rounded on their top corners only, so the baseline stays a straight line.

## Components

### Meter (signature component)
A track at `{colors.surface-2}`, 12px tall, with a fill whose width is the utilisation
percentage and whose colour is the status. Quarter notches are drawn *above* the fill so the
bar can be read on its own. The percentage sits to its left with the word `used` beneath it,
so the row states its own units. The fill transitions width over 0.4s, which covers both the
first paint and every subsequent poll.

### Chart (signature component)
One bar per day, height proportional to that day's tokens against the range peak, in
`{colors.text-dim}` — history is reference material and must not out-shout the meters above
it. Idle days keep a 2% stub in `{colors.border-strong}`, so a quiet week reads as quiet days
rather than as a gap in the axis. The axis carries the first date, the peak value, and the last
date; without the peak the chart says which day was busiest and never how busy.

### Card
`{colors.surface}` on a hairline, 20px padding, heading and origin chip in the header row.

### Origin chip
Tinted border and text, never a filled background: `live` takes the accent, `stale` and
`no reading` take warn, `uncalibrated` takes the neutral border and mute text.

### Lamp (live switch)
A checkbox styled as a bordered pill with a dot. Lit, it takes the accent and reads `Live`; at
rest it reads `Estimated`. It is built once and updated in place — remounting it would take
keyboard focus with it at exactly the moment someone has just operated it.

### Range buttons
Text buttons; the active one takes a `{colors.surface-2}` fill and a `{colors.border-strong}`
edge rather than the accent, so selection does not compete with the live switch.

### Tables
Sentence-case headers in `{typography.caption}`, hairline row rules, no zebra, first column in
`{colors.text}` and figures in mono. Wrapped in an `overflow-x: auto` container.

### Notice
A left rule in `{colors.warn}` on a plain surface. It informs without alarming, which is right
for the system admitting a gap in its own data.

### Motion
One rule on the page: `.meter-fill { transition: width .4s ease }`, disabled under
`prefers-reduced-motion`. There are no keyframes and no entrance animation.

## Do's and don'ts

### Do
- Put the number and the bar in the same row, and let the bar be the redundant one.
- Use the status scale for state and the accent for interaction.
- Set every figure in mono with tabular numerals.
- Say what a reading is when it is not measured — `uncalibrated` is information, not an excuse.
- Distinguish "no block open" from "no fixed reset"; they are different facts.
- Keep new grid tracks at `minmax(0, …)`.

### Don't
- Add a fourth surface level, a shadow, or a gradient.
- Colour a derived reading.
- Spend the accent on anything that is not operable.
- Set a label in uppercase with letterspacing.
- Encode a quantity as lightness, saturation, or texture.
- Add a second large element to the first viewport.
- Introduce a frontend framework, a build step, or a CDN asset. See PRODUCT.md.
