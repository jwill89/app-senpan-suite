/**
 * TypeScript mirror of the Go `internal/model` package.
 *
 * These interfaces are the typed contract between the frontend and the API.
 * Field names match the JSON struct tags on the Go side exactly. When the Go
 * model changes, update this file (or, later, generate it automatically from
 * the Go structs with a tool like tygo so the two cannot drift).
 *
 * @see src/internal/model/model.go
 */

/** A bingo card. board[row][col]; col 0=B(1–15) … col 4=O(61–75); [2][2]=0 (FREE). */
export interface Card {
  id: string
  board_data: number[][]
  player_name: string
  details: string
}

/** A reusable win-pattern template. */
export interface Pattern {
  id: number
  name: string
  pattern_data: boolean[][]
  sort_order: number
  category_id: number
  category_name: string
}

/** Ordered grouping of win patterns. */
export interface PatternCategory {
  id: number
  name: string
  sort_order: number
}

/** Point-in-time snapshot of a pattern attached to a game. */
export interface BingoGamePattern {
  id: number
  name: string
  pattern_data: boolean[][]
}

/** Public view of an active game returned to clients. */
export interface BingoGameState {
  id: number
  called_numbers: number[]
  patterns: BingoGamePattern[]
  total_called: number
}

/** A single drawn number, returned after each draw. */
export interface BingoDrawnNumber {
  number: number
  letter: string
  call_order: number
}

/** A custom CSS theme. */
export interface Style {
  id: number
  name: string
  css_content?: string
  created_at: string
}

/** A raffle event players can enter. */
export interface Raffle {
  id: number
  title: string
  description: string
  rules: string
  max_entries: number
  signup_instructions: string
  cost_per_entry: number
  available_from: string
  available_to: string
  prize_image: string
  status: 'open' | 'closed'
  winner_entry_id: number | null
  created_at: string
}

/** A player's entry (one or more tickets) into a raffle. */
export interface RaffleEntry {
  id: number
  raffle_id: number
  character_name: string
  world: string
  num_entries: number
  paid: boolean
  created_at: string
}

/** A confirmed winner stored in the winners log. */
export interface WinnersLogEntry {
  id: number
  logged_at: string
  card_id: string
  player_name: string
  game_details: string
  winning_patterns: string
}

/** A player who has won multiple times within a recent window. */
export interface FrequentWinner {
  player_name: string
  win_count: number
}

// ── API response envelopes ──────────────────────────────────────────────────

export interface BoardResponse {
  card: Card
  game: BingoGameState | null
  game_details?: string
}

// ── WebSocket message types ─────────────────────────────────────────────────

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
