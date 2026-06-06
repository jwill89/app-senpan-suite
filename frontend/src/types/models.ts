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

import type { BingoDrawnNumber, BingoGameState, Card, Raffle, RaffleEntry } from './generated'

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
