import { useEffect } from 'react'
import { updateActionItemStatus } from '../lib/api'

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

const TAG_COLORS = {
  decision: 'text-blue-300 bg-blue-500/10 border-blue-500/30',
  'action-item': 'text-orange-300 bg-orange-500/10 border-orange-500/30',
  idea: 'text-green-300 bg-green-500/10 border-green-500/30',
  question: 'text-yellow-300 bg-yellow-500/10 border-yellow-500/30',
  document: 'text-purple-300 bg-purple-500/10 border-purple-500/30',
}

export default function MessageDetailDrawer({ item, onClose, onItemChanged }) {
  useEffect(() => {
    function onKey(e) { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

  if (!item) return null

  async function handleStatusChange(e) {
    const newStatus = e.target.value
    try {
      await updateActionItemStatus(item.id, newStatus)
      onItemChanged?.({ ...item, status: newStatus })
    } catch {
      // status stays unchanged; user can retry
    }
  }

  const tagColor = TAG_COLORS[item.ai_type] || ''
  const statusColor = STATUS_COLORS[item.status] || ''
  const timestamp = item.created_at ? new Date(item.created_at).toLocaleString() : '—'

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="flex-1 bg-black/50" onClick={onClose} />
      <div className="w-[420px] max-w-full bg-[#1a1728] border-l border-white/10 overflow-y-auto flex flex-col">
        <div className="p-6 space-y-5 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div className="flex flex-wrap gap-2">
              {item.ai_type && (
                <span className={`text-xs px-2 py-0.5 rounded-full border capitalize ${tagColor}`}>
                  {item.ai_type}
                </span>
              )}
              {item.status && (
                <span className={`text-xs px-2 py-0.5 rounded-full border ${statusColor}`}>
                  {STATUS_LABELS[item.status] || item.status}
                </span>
              )}
            </div>
            <button
              onClick={onClose}
              className="shrink-0 text-gray-500 hover:text-white transition-colors"
              aria-label="Close"
            >
              ✕
            </button>
          </div>

          <div className="space-y-1">
            <p className="text-xs text-gray-500 uppercase tracking-wide">Content</p>
            <p className="text-sm text-gray-200 leading-relaxed whitespace-pre-wrap">{item.text || item.task}</p>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <p className="text-xs text-gray-500 uppercase tracking-wide">From</p>
              <p className="text-sm text-gray-300">{item.sender || '—'}</p>
            </div>
            <div className="space-y-1">
              <p className="text-xs text-gray-500 uppercase tracking-wide">Date</p>
              <p className="text-sm text-gray-300">{timestamp}</p>
            </div>
          </div>

          {item.topic && (
            <div className="space-y-1">
              <p className="text-xs text-gray-500 uppercase tracking-wide">Topic</p>
              <p className="text-sm text-gray-300">{item.topic}</p>
            </div>
          )}

          {item.due_date && (
            <div className="space-y-1">
              <p className="text-xs text-gray-500 uppercase tracking-wide">Due Date</p>
              <p className="text-sm text-gray-300">{item.due_date}</p>
            </div>
          )}

          {item.status && (
            <div className="space-y-1">
              <p className="text-xs text-gray-500 uppercase tracking-wide">Change Status</p>
              <select
                value={item.status}
                onChange={handleStatusChange}
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-violet-500"
              >
                <option value="open">Open</option>
                <option value="in_progress">In Progress</option>
                <option value="done">Done</option>
                <option value="cancelled">Cancelled</option>
              </select>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
