import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { listActionItems, listMessagesByTag, updateActionItemStatus } from '../lib/api'
import MessageDetailDrawer from './MessageDetailDrawer'

const PAGE_SIZE = 20

const STATUS_LABELS = {
  open: 'Open',
  in_progress: 'In Progress',
  done: 'Done',
  cancelled: 'Cancelled',
}

const STATUS_COLORS = {
  open: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30',
  in_progress: 'text-blue-400 bg-blue-500/10 border-blue-500/30',
  done: 'text-gray-400 bg-gray-500/10 border-gray-500/30',
  cancelled: 'text-red-400 bg-red-500/10 border-red-500/30',
}

// Groups for Actions page
const ACTION_GROUPS = [
  { key: 'open', label: 'Open' },
  { key: 'in_progress', label: 'In Progress' },
  { key: 'done', label: 'Done' },
  { key: 'cancelled', label: 'Cancelled' },
]

function normalizeMessage(m) {
  return {
    ...m,
    text: m.text,
    sender: m.sender_id ? `User ${m.sender_id}` : '—',
    created_at: m.created_at,
    ai_type: m.ai_type,
    topic: m.topic || null,
    status: null,
    _isActionItem: false,
  }
}

function normalizeActionItem(item) {
  return {
    ...item,
    text: item.task,
    sender: item.owner || '—',
    created_at: item.created_at || null,
    ai_type: 'action-item',
    topic: null,
    status: item.status,
    _isActionItem: true,
  }
}

function StatusBadge({ status }) {
  if (!status) return null
  return (
    <span className={`text-xs px-2 py-0.5 rounded-full border shrink-0 ${STATUS_COLORS[status] || 'text-gray-400 bg-gray-500/10 border-gray-500/30'}`}>
      {STATUS_LABELS[status] || status}
    </span>
  )
}

function InlineStatusSelect({ item, onChange }) {
  function handleChange(e) {
    e.stopPropagation()
    onChange(item.id, e.target.value)
  }
  return (
    <select
      value={item.status}
      onChange={handleChange}
      onClick={e => e.stopPropagation()}
      className="shrink-0 bg-white/5 border border-white/10 rounded px-2 py-1 text-xs text-gray-300 focus:outline-none focus:border-violet-500"
    >
      <option value="open">Open</option>
      <option value="in_progress">In Progress</option>
      <option value="done">Done</option>
      <option value="cancelled">Cancelled</option>
    </select>
  )
}

function ItemCard({ item, showStatus, onSelect, onStatusChange }) {
  const ts = item.created_at ? new Date(item.created_at).toLocaleString() : '—'
  return (
    <li
      onClick={() => onSelect(item)}
      className="rounded-lg border border-white/10 bg-white/2 p-4 cursor-pointer hover:border-white/20 hover:bg-white/5 transition-colors"
    >
      <p className="text-sm text-gray-200 leading-relaxed mb-3">{item.text}</p>
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-3 text-xs text-gray-500 min-w-0">
          <span className="truncate">{item.sender}</span>
          <span className="shrink-0">{ts}</span>
        </div>
        {showStatus && item.status && (
          <div className="flex items-center gap-2" onClick={e => e.stopPropagation()}>
            <StatusBadge status={item.status} />
            <InlineStatusSelect item={item} onChange={onStatusChange} />
          </div>
        )}
      </div>
    </li>
  )
}

function CollapsibleGroup({ label, count, defaultOpen = true, children }) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div>
      <button
        onClick={() => setOpen(o => !o)}
        className="flex items-center gap-2 w-full text-left py-2 mb-1"
      >
        <span className="text-xs font-semibold uppercase tracking-wider text-gray-400">{label}</span>
        <span className="text-xs text-gray-600">({count})</span>
        <span className="text-gray-600 ml-auto text-xs">{open ? '▼' : '▶'}</span>
      </button>
      {open && children}
    </div>
  )
}

export default function ItemListPage({ title, tag, showStatus }) {
  const { chatId } = useParams()
  const isActionItems = tag === 'action-item'

  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState('')
  const [hasMore, setHasMore] = useState(false)
  const [beforeId, setBeforeId] = useState(0)
  const [selectedItem, setSelectedItem] = useState(null)

  // Filters
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')

  useEffect(() => {
    setLoading(true)
    setItems([])
    setBeforeId(0)
    setHasMore(false)
    setError('')

    if (isActionItems) {
      listActionItems(chatId)
        .then(data => setItems(data.map(normalizeActionItem)))
        .catch(err => setError(err.message))
        .finally(() => setLoading(false))
    } else {
      listMessagesByTag(chatId, tag)
        .then(data => {
          const normalized = data.map(normalizeMessage)
          setItems(normalized)
          setHasMore(data.length === PAGE_SIZE)
          if (data.length > 0) setBeforeId(data[data.length - 1].id)
        })
        .catch(err => setError(err.message))
        .finally(() => setLoading(false))
    }
  }, [chatId, tag])

  async function loadMore() {
    if (loadingMore || !hasMore) return
    setLoadingMore(true)
    try {
      const data = await listMessagesByTag(chatId, tag, beforeId)
      const normalized = data.map(normalizeMessage)
      setItems(prev => [...prev, ...normalized])
      setHasMore(data.length === PAGE_SIZE)
      if (data.length > 0) setBeforeId(data[data.length - 1].id)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoadingMore(false)
    }
  }

  async function handleStatusChange(id, status) {
    try {
      await updateActionItemStatus(id, status)
      setItems(prev => prev.map(it => it.id === id ? { ...it, status } : it))
      setSelectedItem(prev => prev?.id === id ? { ...prev, status } : prev)
    } catch (err) {
      setError(err.message)
    }
  }

  function handleItemChanged(updated) {
    setItems(prev => prev.map(it => it.id === updated.id ? { ...it, ...updated } : it))
    setSelectedItem(updated)
  }

  function clearFilters() {
    setDateFrom('')
    setDateTo('')
    setStatusFilter('all')
  }

  const filtered = items.filter(item => {
    if (dateFrom && item.created_at && item.created_at.slice(0, 10) < dateFrom) return false
    if (dateTo && item.created_at && item.created_at.slice(0, 10) > dateTo) return false
    if (showStatus && statusFilter !== 'all' && item.status !== statusFilter) return false
    return true
  })

  function renderList() {
    if (filtered.length === 0) {
      return (
        <p className="text-gray-500 py-8 text-center text-sm">
          No {title.toLowerCase()} recorded yet.
        </p>
      )
    }

    if (isActionItems) {
      return (
        <div className="space-y-6">
          {ACTION_GROUPS.map(group => {
            const groupItems = filtered.filter(it => it.status === group.key)
            if (groupItems.length === 0) return null
            return (
              <CollapsibleGroup
                key={group.key}
                label={group.label}
                count={groupItems.length}
                defaultOpen={group.key === 'open' || group.key === 'in_progress'}
              >
                <ul className="space-y-2 mt-1">
                  {groupItems.map(item => (
                    <ItemCard
                      key={item.id}
                      item={item}
                      showStatus={showStatus}
                      onSelect={setSelectedItem}
                      onStatusChange={handleStatusChange}
                    />
                  ))}
                </ul>
              </CollapsibleGroup>
            )
          })}
        </div>
      )
    }

    if (tag === 'decision') {
      return (
        <CollapsibleGroup label="Active" count={filtered.length} defaultOpen>
          <ul className="space-y-2 mt-1">
            {filtered.map(item => (
              <ItemCard
                key={item.id}
                item={item}
                showStatus={false}
                onSelect={setSelectedItem}
                onStatusChange={handleStatusChange}
              />
            ))}
          </ul>
        </CollapsibleGroup>
      )
    }

    return (
      <ul className="space-y-2">
        {filtered.map(item => (
          <ItemCard
            key={item.id}
            item={item}
            showStatus={false}
            onSelect={setSelectedItem}
            onStatusChange={handleStatusChange}
          />
        ))}
      </ul>
    )
  }

  const hasFilters = dateFrom || dateTo || statusFilter !== 'all'

  return (
    <div className="min-h-screen bg-[#15131f] px-6 py-12 text-white">
      <div className="mx-auto max-w-3xl space-y-6">
        <div className="flex items-center gap-4">
          <Link
            to={`/dashboard/chats/${chatId}`}
            className="text-sm text-gray-500 hover:text-violet-300 transition-colors"
          >
            ← Back
          </Link>
          <h1 className="text-2xl font-bold">{title}</h1>
        </div>

        {/* Filter bar */}
        <div className="flex flex-wrap items-end gap-3 rounded-lg border border-white/10 bg-white/3 p-4">
          <div className="space-y-1">
            <label className="text-xs text-gray-500 uppercase tracking-wide">From</label>
            <input
              type="date"
              value={dateFrom}
              onChange={e => setDateFrom(e.target.value)}
              className="block rounded border border-white/10 bg-white/5 px-2 py-1.5 text-sm text-white focus:outline-none focus:border-violet-500"
            />
          </div>
          <div className="space-y-1">
            <label className="text-xs text-gray-500 uppercase tracking-wide">To</label>
            <input
              type="date"
              value={dateTo}
              onChange={e => setDateTo(e.target.value)}
              className="block rounded border border-white/10 bg-white/5 px-2 py-1.5 text-sm text-white focus:outline-none focus:border-violet-500"
            />
          </div>
          {showStatus && (
            <div className="space-y-1">
              <label className="text-xs text-gray-500 uppercase tracking-wide">Status</label>
              <select
                value={statusFilter}
                onChange={e => setStatusFilter(e.target.value)}
                className="block rounded border border-white/10 bg-white/5 px-2 py-1.5 text-sm text-white focus:outline-none focus:border-violet-500"
              >
                <option value="all">All</option>
                <option value="open">Open</option>
                <option value="in_progress">In Progress</option>
                <option value="done">Done</option>
                <option value="cancelled">Cancelled</option>
              </select>
            </div>
          )}
          {hasFilters && (
            <button
              onClick={clearFilters}
              className="rounded border border-white/10 px-3 py-1.5 text-xs text-gray-400 hover:text-white hover:border-white/30 transition-colors"
            >
              Clear filters
            </button>
          )}
          <span className="ml-auto text-xs text-gray-600">{filtered.length} items</span>
        </div>

        {error && <p className="text-red-400 text-sm">{error}</p>}

        {loading ? (
          <p className="text-gray-500 py-8 text-center text-sm">Loading…</p>
        ) : (
          <>
            {renderList()}

            {hasMore && !isActionItems && (
              <div className="flex justify-center pt-4">
                <button
                  onClick={loadMore}
                  disabled={loadingMore}
                  className="rounded-lg border border-white/10 px-6 py-2 text-sm text-gray-300 hover:border-white/30 hover:text-white disabled:opacity-50 transition-colors"
                >
                  {loadingMore ? 'Loading…' : 'Load more'}
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {selectedItem && (
        <MessageDetailDrawer
          item={selectedItem}
          onClose={() => setSelectedItem(null)}
          onItemChanged={handleItemChanged}
        />
      )}
    </div>
  )
}
