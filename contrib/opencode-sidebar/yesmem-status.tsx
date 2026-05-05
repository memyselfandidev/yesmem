/** @jsxImportSource @opentui/solid */

import type { TuiPlugin, TuiPluginModule, TuiSlotPlugin } from "@opencode-ai/plugin/tui"
import type { JSX } from "@opentui/solid"
import { readFileSync } from "node:fs"
import { homedir } from "node:os"
import { join } from "node:path"
import { readdirSync, existsSync } from "node:fs"

type CacheStatus = {
  ttl: string
  ttl_seconds: number
  remaining_s: number
  cache_state: "warm" | "cooling" | "cold"
  cost_per_req: number
  last_request_ts: number
  total_tokens: number
  raw_token_estimate: number
  cache_read_tokens: number
  cache_write_tokens: number
  thread_id?: string
  token_threshold: number
  token_minimum_threshold: number
  detected_ttl: string
  keepalive_mode: string
  ping_interval_s: number
  pings_remaining: number
  active_threads: number
}

type StatusState = {
  status: CacheStatus | null
  error: string | null
  lastRefresh: number
  sessionId: string
}

const yesmemDataDir = join(homedir(), ".claude", "yesmem")
const cacheStatusDir = join(yesmemDataDir, "cache-status")

function readStatus(sessionId: string): CacheStatus | null {
  try {
    const direct = join(cacheStatusDir, `status-${sessionId}.json`)
    if (existsSync(direct)) {
      return JSON.parse(readFileSync(direct, "utf-8")) as CacheStatus
    }
    if (existsSync(cacheStatusDir)) {
      const files = readdirSync(cacheStatusDir).filter((f) => f.startsWith("status-") && f.endsWith(".json"))
      for (const f of files.sort().reverse()) {
        try {
          return JSON.parse(readFileSync(join(cacheStatusDir, f), "utf-8")) as CacheStatus
        } catch { continue }
      }
    }
  } catch { /* ignore */ }
  return null
}

function formatInterval(totalS: number): string {
  const m = Math.floor(totalS / 60)
  const s = totalS % 60
  if (m === 0) return `${s}s`
  if (s === 0) return `${m}m`
  return `${m}m${s}s`
}

function ttlLabel(detected: string): string {
  switch (detected) {
    case "1h": return "1h"
    case "5min": return "5min"
    default: return "detecting…"
  }
}

const color = {
  green:  "#4ade80",
  yellow: "#facc15",
  red:    "#f87171",
  muted:  "#9ca3af",
  cyan:   "#22d3ee",
  blue:   "#60a5fa",
}

// ── sidebar_content slot ─────────────────────────────────────────────

const sidebar: TuiSlotPlugin = {
  order: 650,
  slots: {
    sidebar_content(ctx, value) {
      const theme = ctx.theme.current

      const status = readStatus(value.session_id)
      if (!status) {
        return (
          <box
            border borderColor={color.muted}
            backgroundColor={theme.backgroundPanel}
            paddingTop={1} paddingBottom={1}
            paddingLeft={2} paddingRight={2}
            flexDirection="column" gap={1}
          >
            <text fg={color.muted}>yesmem</text>
            <text fg={color.muted}>waiting for cache data…</text>
          </box>
        )
      }

      const tokK = Math.floor(status.total_tokens / 1000)
      const readK = Math.floor(status.cache_read_tokens / 1000)
      const writeK = Math.floor(status.cache_write_tokens / 1000)
      const uncached = status.total_tokens - status.cache_read_tokens - status.cache_write_tokens
      const uncachedK = Math.floor(uncached / 1000)
      const hitPct = status.total_tokens > 0
        ? Math.floor(status.cache_read_tokens * 100 / status.total_tokens)
        : 0

      const reqCost = (status.cache_read_tokens * 0.30 +
                       status.cache_write_tokens * 3.75 +
                       uncached * 3.0) / 1_000_000

      const stateColor = status.cache_state === "warm" ? color.green
        : status.cache_state === "cooling" ? color.yellow
        : color.red

      const hitColor = hitPct >= 50 ? color.green : color.red

      // Cache lifeline
      let cacheLine: string
      if (status.cache_state === "cold") {
        const thresholdK = status.token_threshold > 0 ? `/${Math.floor(status.token_threshold / 1000)}k` : ""
        cacheLine = `COLD (${ttlLabel(status.detected_ttl)}) | ${tokK}k${thresholdK} Token`
      } else {
        const expiry = new Date((status.last_request_ts + status.ttl_seconds) * 1000)
        const expiryStr = expiry.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" })
        let line = `Cache (${ttlLabel(status.detected_ttl)}) until ${expiryStr}`

        if (status.pings_remaining > 0 && status.ping_interval_s > 0) {
          const kaEnd = new Date(expiry.getTime() + status.pings_remaining * status.ping_interval_s * 1000)
          const kaStr = kaEnd.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" })
          line += ` | Keepalive until ${kaStr}`
          line += ` (${status.pings_remaining} ping${status.pings_remaining !== 1 ? "s" : ""} / ${formatInterval(status.ping_interval_s)})`
        }
        cacheLine = line
      }

      // Collapsing line
      let collapseLine = ""
      if (status.token_threshold > 0 && status.token_minimum_threshold > 0) {
        const rawEstimate = status.raw_token_estimate
        if (rawEstimate > 0 && rawEstimate > status.total_tokens) {
          const rawK = Math.floor(rawEstimate * 1.19 / 1000)
          const saved = 100 - Math.floor(tokK * 100 / rawK)
          collapseLine = `Collapsing ${rawK}k → ${tokK}k (${saved}% saved) | threshold: ${Math.floor(status.token_threshold / 1000)}k`
        } else {
          const threshK = Math.floor(status.token_threshold / 1000)
          const minK = Math.floor(status.token_minimum_threshold / 1000)
          collapseLine = `Collapsing at ${threshK}k → ${minK}k | actual: ${tokK}k / ${threshK}k`
        }
      }

      // Cold cost
      const coldCost = (tokK * (status.detected_ttl === "1h" ? 0.01 : 0.00625))

      return (
        <box
          border borderColor={color.muted}
          backgroundColor={theme.backgroundPanel}
          paddingTop={1} paddingBottom={1}
          paddingLeft={2} paddingRight={2}
          flexDirection="column" gap={1}
        >
          <box flexDirection="row" gap={1}>
            <text fg={color.blue}><b>yesmem</b></text>
            {status.thread_id ? (
              <text fg={color.muted}>{status.thread_id.slice(0, 12)}</text>
            ) : null}
          </box>

          <text fg={stateColor}><b>{cacheLine}</b></text>

          {collapseLine ? <text fg={color.muted}>{collapseLine}</text> : null}

          <box flexDirection="row" gap={1}>
            <text fg={color.muted}>R:</text><text fg={color.green}>{readK}k</text>
            <text fg={color.muted}>W:</text><text fg={color.yellow}>{writeK}k</text>
            <text fg={color.muted}>U:</text><text fg={color.muted}>{uncachedK}k</text>
            <text fg={hitColor}>({hitPct}% hit)</text>
          </box>

          <box flexDirection="row" gap={1}>
            <text fg={color.muted}>last:</text>
            <text fg={hitColor}>${reqCost.toFixed(2)}</text>
            <text fg={color.muted}>| after expiry:</text>
            <text fg={color.red}>${coldCost.toFixed(2)}</text>
          </box>

          {status.active_threads > 0 ? (
            <text fg={color.muted}>
              {status.keepalive_mode} keepalive · {status.active_threads} thread{status.active_threads !== 1 ? "s" : ""}
            </text>
          ) : null}
        </box>
      )
    },
  },
}

// ── sidebar_footer slot ──────────────────────────────────────────────

const footer: TuiSlotPlugin = {
  order: 0,
  slots: {
    sidebar_footer(ctx, value) {
      const theme = ctx.theme.current
      const status = readStatus(value.session_id)
      if (!status || status.cache_state !== "cold") return null

      return (
        <box
          backgroundColor={theme.backgroundPanel}
          paddingTop={1} paddingBottom={1}
          paddingLeft={2} paddingRight={2}
        >
          <text fg={color.red}><b>⚠ yesmem cache cold</b></text>
        </box>
      )
    },
  },
}

// ── plugin entry ─────────────────────────────────────────────────────

const tui: TuiPlugin = async (api) => {
  api.slots.register(sidebar)
  api.slots.register(footer)
}

const plugin: TuiPluginModule & { id: string } = {
  id: "yesmem-status",
  tui,
}

export default plugin
