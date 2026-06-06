/**
 * Frontend type contract.
 *
 * The domain types (Card, Pattern, Raffle, BingoGameState, …) are GENERATED
 * from the Go `internal/model` package — see ./generated.ts. Do not edit that
 * file; regenerate with `cd src && go generate ./...`.
 *
 * This module re-exports the generated types and adds the shapes that have no
 * Go struct counterpart: API response envelopes and WebSocket messages, which
 * the Go handlers construct inline as anonymous structs / maps. Keep these in
 * sync with the relevant handlers by hand.
 */
export * from './generated'

import type {
  BingoDrawnNumber,
  BingoGameState,
  Card,
  FrequentWinner,
  Pattern,
  PatternCategory,
  Raffle,
  RaffleEntry,
  WinnersLogEntry,
} from './generated'

// ── API response envelopes ──────────────────────────────────────────────────

export interface BoardResponse {
  card: Card
  game: BingoGameState | null
  game_details?: string
}

export interface RafflesListResponse {
  raffles: Raffle[]
}

export interface RaffleDetailResponse {
  raffle: Raffle
  total_entries?: number
  /** Present for the public view of a closed raffle that has a winner. */
  winner_entry?: RaffleEntry
  /** Present only for admins. */
  entries?: RaffleEntry[]
}

export interface RaffleEnterResponse {
  message: string
  total_entries: number
  total_cost: number
  signup_instructions: string
}

// ── Admin endpoints ─────────────────────────────────────────────────────────

export interface AuthCheckResponse {
  authenticated: boolean
}

/** Editable app settings (all values are strings; see internal/server/settings.go). */
export interface AppSettings {
  app_title: string
  default_draw_delay: string
  frequent_winner_threshold: string
  frequent_winner_hours: string
  header_font: string
  google_fonts_api_key: string
}

export interface SettingsResponse {
  settings: AppSettings
}

/** GET /api/cards returns IDs + names only (no board data). */
export type AdminCardSummary = Pick<Card, 'id' | 'player_name' | 'details'>

export interface AdminCardsResponse {
  cards: AdminCardSummary[]
}

export interface GenerateCardsResponse {
  cards: Array<Pick<Card, 'id' | 'board_data'>>
  count: number
}

export interface WinnersLogResponse {
  entries: WinnersLogEntry[]
  total: number
  page: number
  per_page: number
}

/** GET /api/game and the "start" action response. */
export interface GameStateResponse {
  game: BingoGameState | null
  winners: string[]
  game_details: string
}

/** POST /api/game {action:"draw"} response (DrawResult). */
export interface DrawResultResponse {
  drawn: BingoDrawnNumber
  game: BingoGameState
  winners: string[]
}

export interface PatternsListResponse {
  patterns: Pattern[]
  categories: PatternCategory[]
}

export interface FrequentWinnersResponse {
  winners: FrequentWinner[]
}

// ── WebSocket message types ─────────────────────────────────────────────────
// Mirror the broadcast payloads in internal/server (server.go, game.go).

export interface WSGameUpdate {
  type: 'game_update'
  game: BingoGameState | null
  game_details?: string
  winners?: string[]
}

export interface WSGameDraw {
  type: 'game_draw'
  drawn: BingoDrawnNumber
  winners?: string[]
}

export interface WSCardDeleted {
  type: 'card_deleted'
}

export interface WSDetailsUpdate {
  type: 'details_update'
  game_details: string
}

export interface WSStyleUpdate {
  type: 'style_update'
  css: string
}

export interface WSHalftime {
  type: 'halftime_minigame'
}

/** Discriminated union of every message the server can push over the socket. */
export type WSMessage =
  | WSGameUpdate
  | WSGameDraw
  | WSCardDeleted
  | WSDetailsUpdate
  | WSStyleUpdate
  | WSHalftime
