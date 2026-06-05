/**
 * Real-time game socket as a reusable composable.
 *
 * In the legacy app.js this logic (connect, exponential-backoff reconnect,
 * 25s keepalive ping, message dispatch) was ~120 lines spread across the single
 * giant component. Here it is one self-contained, typed unit that any view can
 * consume — the core argument for the component/composable refactor.
 */
import { ref, type Ref } from 'vue'
import type { WSMessage } from '@/types/models'

const KEEPALIVE_MS = 25_000
const MAX_RECONNECT = 10
const MAX_BACKOFF_MS = 16_000

export interface GameSocketHandlers {
  onMessage: (msg: WSMessage) => void
  /** Called on each successful (re)connect after the first, to resync state. */
  onReconnect?: () => void
}

export interface GameSocket {
  connected: Ref<boolean>
  connect: () => void
  close: () => void
}

/**
 * @param cardId Reactive card ID; appended as `?id=` so the server can target
 *               this player (e.g. to disconnect them when their card is deleted).
 */
export function useGameSocket(cardId: Ref<string | null>, handlers: GameSocketHandlers): GameSocket {
  const connected = ref(false)

  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let keepaliveTimer: ReturnType<typeof setInterval> | null = null
  let attempts = 0
  let manualClose = false

  function url(): string {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    let u = `${proto}//${location.host}/api/ws`
    if (cardId.value) u += `?id=${encodeURIComponent(cardId.value)}`
    return u
  }

  function startKeepalive() {
    stopKeepalive()
    keepaliveTimer = setInterval(() => {
      if (ws?.readyState === WebSocket.OPEN) ws.send('ping')
    }, KEEPALIVE_MS)
  }

  function stopKeepalive() {
    if (keepaliveTimer) clearInterval(keepaliveTimer)
    keepaliveTimer = null
  }

  function scheduleReconnect() {
    if (reconnectTimer || manualClose) return
    attempts++
    if (attempts > MAX_RECONNECT) return
    const delay = Math.min(1000 * 2 ** (attempts - 1), MAX_BACKOFF_MS)
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, delay)
  }

  function connect() {
    manualClose = false
    const isReconnect = attempts > 0
    const sock = new WebSocket(url())
    ws = sock

    sock.onopen = () => {
      connected.value = true
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
      if (isReconnect) handlers.onReconnect?.()
      attempts = 0
      startKeepalive()
    }

    sock.onmessage = (evt: MessageEvent<string>) => {
      let msg: WSMessage
      try {
        msg = JSON.parse(evt.data) as WSMessage
      } catch {
        return
      }
      handlers.onMessage(msg)
    }

    sock.onclose = () => {
      connected.value = false
      stopKeepalive()
      if (ws === sock) ws = null
      scheduleReconnect()
    }

    sock.onerror = () => {
      // onclose fires after onerror; reconnect is handled there.
    }
  }

  function close() {
    manualClose = true
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = null
    stopKeepalive()
    attempts = 0
    connected.value = false
    if (ws) {
      ws.onclose = null // prevent reconnect loop
      ws.close()
      ws = null
    }
  }

  return { connected, connect, close }
}
