import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'

import { listMessages } from '../lib/api'

// ---- Tag colour mapping -------------------------------------------------

const TAG_STYLES = {
  decision: {
    border: 'border-blue-500',
    pill: 'bg-blue-500/20 text-blue-300',
    label: 'decision',
  },
  'action-item': {
    border: 'border-orange-500',
    pill: 'bg-orange-500/20 text-orange-300',
    label: 'action item',
  },
  idea: {
    border: 'border-green-500',
    pill: 'bg-green-500/20 text-green-300',
    label: 'idea',
  },
  question: {
    border: 'border-yellow-500',
    pill: 'bg-yellow-500/20 text-yellow-300',
    label: 'question',
  },
  document: {
    border: 'border-purple-500',
    pill: 'bg-purple-500/20 text-purple-300',
    label: 'document',
  },
}

const DEFAULT_STYLE = {
  border: 'border-white/20',
  pill: 'bg-white/10 text-gray-400',
  label: '',
}

function tagStyle(tag) {
  return TAG_STYLES[tag] || DEFAULT_STYLE
}

// ---- Date helpers -------------------------------------------------------

function formatDayHeading(dateStr) {
  // dateStr is a YYYY-MM-DD string (local day key)
  const [year, month, day] = dateStr.split('-').map(Number)
  const d = new Date(year, month - 1, day)
  return d.toLocaleDateString('en-US', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
}

function toLocalDayKey(isoString) {
  const d = new Date(isoString)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function formatTime(isoString) {
  const d = new Date(isoString)
  return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })
}

// ---- Status badge -------------------------------------------------------

function StatusBadge({ tag, topic }) {
  if (!topic && tag !== 'decision' && tag !== 'action-item') return null
  return (
    <span className="ml-2 rounded px-1.5 py-0.5 text-[10px] font-medium bg-white/10 text-gray-400">
      {topic || (tag === 'decision' ? 'decided' : 'open')}
    </span>
  )
}

// ---- Detail drawer ------------------------------------------------------

function DetailDrawer({ message, onClose }) {
  const style = tagStyle(message.ai_type)

  // Close on Escape key
  useEffect(() => {
    function onKey(e) {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-end"
      onClick={onClose}
      aria-modal="true"
      role="dialog"
    >
      {/* backdrop */}
      <div className="absolute inset-0 bg-black/50" />

      {/* panel */}
      <div
        className="relative z-10 h-full w-full max-w-md overflow-y-auto bg-[#1e1b2e] shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-white/10 px-6 py-4">
          <span className={`rounded-full px-2.5 py-0.5 text-xs font-semibold ${style.pill}`}>
            {style.label || message.ai_type}
          </span>
          <button
            onClick={onClose}
            className="rounded-full p-1.5 text-gray-400 hover:bg-white/10 hover:text-white"
          >
            ✕
          </button>
        </div>

        <div className="space-y-4 px-6 py-5">
          <div className="flex items-center gap-3 text-sm text-gray-400">
            <span className="font-medium text-gray-200">#{message.sender_id}</span>
            <span>·</span>
            <span>{new Date(message.created_at).toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' })}</span>
          </div>

          {message.topic && (
            <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
              {message.topic}
            </p>
          )}

          <p className="text-sm leading-relaxed text-gray-100">{message.text}</p>

          {message.is_voice && (
            <p className="text-xs text-gray-500 italic">Voice message (transcribed)</p>
          )}
        </div>
      </div>
    </div>
  )
}

// ---- Message card -------------------------------------------------------

function MessageCard({ message, onClick }) {
  const style = tagStyle(message.ai_type)
  return (
    <button
      onClick={() => onClick(message)}
      className={`w-full text-left rounded-lg border border-white/10 border-l-2 ${style.border} bg-white/[0.03] p-3 transition hover:bg-white/[0.07] focus:outline-none focus-visible:ring-2 focus-visible:ring-violet-500`}
    >
      <div className="flex flex-wrap items-center gap-2">
        <span className={`shrink-0 rounded-full px-2 py-0.5 text-[11px] font-semibold ${style.pill}`}>
          {style.label || message.ai_type}
        </span>
        <span className="text-xs text-gray-500">#{message.sender_id}</span>
        <span className="text-xs text-gray-600">{formatTime(message.created_at)}</span>
        {(message.ai_type === 'decision' || message.ai_type === 'action-item') && (
          <StatusBadge tag={message.ai_type} topic={message.topic} />
        )}
      </div>
      <p className="mt-2 text-sm leading-snug text-gray-200 line-clamp-3">{message.text}</p>
    </button>
  )
}

// ---- Day section --------------------------------------------------------

function DaySection({ dayKey, messages, onMessageClick }) {
  return (
    <section>
      <h2 className="sticky top-0 z-10 bg-[#15131f]/90 py-2 text-xs font-semibold uppercase tracking-widest text-gray-500 backdrop-blur-sm">
        {formatDayHeading(dayKey)}
      </h2>
      <ul className="mt-2 space-y-2">
        {messages.map((msg) => (
          <li key={msg.id}>
            <MessageCard message={msg} onClick={onMessageClick} />
          </li>
        ))}
      </ul>
    </section>
  )
}

// ---- Group messages by day, preserving day-desc / within-day-asc -------

function groupByDay(messages) {
  // messages arrive newest-first from the API; we want sections newest-day
  // first, but within each day chronological (oldest first).
  const dayMap = new Map()
  for (const msg of messages) {
    const key = toLocalDayKey(msg.created_at)
    if (!dayMap.has(key)) dayMap.set(key, [])
    dayMap.get(key).push(msg)
  }
  // Within each day, reverse to chronological order
  for (const msgs of dayMap.values()) {
    msgs.sort((a, b) => new Date(a.created_at) - new Date(b.created_at))
  }
  // Sort days newest first
  const sortedKeys = [...dayMap.keys()].sort((a, b) => (a > b ? -1 : 1))
  return sortedKeys.map((key) => ({ dayKey: key, messages: dayMap.get(key) }))
}

// ---- Main Timeline page -------------------------------------------------

export default function Timeline() {
  const { chatId } = useParams()

  const [allMessages, setAllMessages] = useState([])
  const [loading, setLoading] = useState(false)
  const [hasMore, setHasMore] = useState(true)
  const [error, setError] = useState('')

  const [searchText, setSearchText] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')

  const [selectedMessage, setSelectedMessage] = useState(null)

  // Sentinel ref for IntersectionObserver
  const sentinelRef = useRef(null)
  // Track whether a load is in-flight so the observer doesn't double-fire
  const loadingRef = useRef(false)

  // Debounce search input by 300 ms
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(searchText), 300)
    return () => clearTimeout(timer)
  }, [searchText])

  // Load a page of messages. beforeId=0 means "start from newest".
  const loadPage = useCallback(
    async (beforeId) => {
      if (loadingRef.current) return
      loadingRef.current = true
      setLoading(true)
      setError('')
      try {
        const page = await listMessages(chatId, beforeId)
        if (!page || page.length === 0) {
          setHasMore(false)
          return
        }
        setAllMessages((prev) => {
          // Deduplicate by id (shouldn't be needed but safe)
          const ids = new Set(prev.map((m) => m.id))
          const fresh = page.filter((m) => !ids.has(m.id))
          return [...prev, ...fresh]
        })
        if (page.length < 50) {
          setHasMore(false)
        }
      } catch (err) {
        setError(err.message)
      } finally {
        loadingRef.current = false
        setLoading(false)
      }
    },
    [chatId],
  )

  // Initial load
  useEffect(() => {
    setAllMessages([])
    setHasMore(true)
    loadingRef.current = false
    loadPage(0)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chatId])

  // IntersectionObserver for infinite scroll
  useEffect(() => {
    if (!sentinelRef.current) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !loadingRef.current) {
          const minId = allMessages.length > 0 ? Math.min(...allMessages.map((m) => m.id)) : 0
          loadPage(minId)
        }
      },
      { rootMargin: '200px' },
    )
    observer.observe(sentinelRef.current)
    return () => observer.disconnect()
  }, [allMessages, hasMore, loadPage])

  // Apply client-side text filter
  const filtered = debouncedSearch
    ? allMessages.filter((m) =>
        m.text.toLowerCase().includes(debouncedSearch.toLowerCase()),
      )
    : allMessages

  const grouped = groupByDay(filtered)

  return (
    <div className="min-h-screen bg-[#15131f] text-white">
      {/* Header */}
      <div className="sticky top-0 z-20 border-b border-white/10 bg-[#15131f]/95 backdrop-blur-sm">
        <div className="mx-auto flex max-w-2xl items-center gap-4 px-6 py-4">
          <Link
            to={`/dashboard/chats/${chatId}`}
            className="shrink-0 text-sm text-violet-300 hover:underline"
          >
            ← Chat
          </Link>
          <h1 className="mr-auto text-base font-semibold">Timeline</h1>
          <input
            type="search"
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            placeholder="Filter messages…"
            className="w-48 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm placeholder-gray-600 focus:border-violet-500 focus:outline-none sm:w-64"
          />
        </div>
      </div>

      {/* Body */}
      <div className="mx-auto max-w-2xl space-y-8 px-6 py-8">
        {error && <p className="rounded-lg bg-red-500/10 px-4 py-3 text-sm text-red-400">{error}</p>}

        {grouped.length === 0 && !loading && (
          <p className="text-center text-gray-500">
            {debouncedSearch ? 'No messages match your filter.' : 'No tagged messages yet.'}
          </p>
        )}

        {grouped.map(({ dayKey, messages }) => (
          <DaySection
            key={dayKey}
            dayKey={dayKey}
            messages={messages}
            onMessageClick={setSelectedMessage}
          />
        ))}

        {/* Infinite scroll sentinel */}
        <div ref={sentinelRef} className="h-4" />

        {loading && (
          <p className="pb-8 text-center text-sm text-gray-500">Loading…</p>
        )}
        {!hasMore && allMessages.length > 0 && !debouncedSearch && (
          <p className="pb-8 text-center text-xs text-gray-600">All messages loaded</p>
        )}
      </div>

      {/* Detail drawer */}
      {selectedMessage && (
        <DetailDrawer message={selectedMessage} onClose={() => setSelectedMessage(null)} />
      )}
    </div>
  )
}
