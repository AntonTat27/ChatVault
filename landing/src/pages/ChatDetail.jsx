import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'

import { listActionItems, listSummaries, searchMessages, updateActionItemStatus } from '../lib/api'

export default function ChatDetail() {
  const { chatId } = useParams()
  const [summaries, setSummaries] = useState([])
  const [actionItems, setActionItems] = useState([])
  const [query, setQuery] = useState('')
  const [results, setResults] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    setError('')
    Promise.all([listSummaries(chatId), listActionItems(chatId)])
      .then(([summaryList, items]) => {
        setSummaries(summaryList)
        setActionItems(items)
      })
      .catch((err) => setError(err.message))
  }, [chatId])

  async function handleToggleDone(item) {
    const nextStatus = item.status === 'done' ? 'open' : 'done'
    await updateActionItemStatus(item.id, nextStatus)
    setActionItems((items) => items.map((i) => (i.id === item.id ? { ...i, status: nextStatus } : i)))
  }

  async function handleSearch(e) {
    e.preventDefault()
    if (!query.trim()) return
    setResults(await searchMessages(chatId, query))
  }

  return (
    <div className="min-h-screen bg-[#15131f] px-6 py-12 text-white">
      <div className="mx-auto max-w-3xl space-y-10">
        <Link to={`/dashboard/chats/${chatId}/integrations`} className="text-sm text-violet-300 hover:underline">
          Notion integration →
        </Link>
        {error && <p className="text-red-400">{error}</p>}

        <section>
          <h2 className="text-xl font-semibold">Search</h2>
          <form onSubmit={handleSearch} className="mt-3 flex gap-2">
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search messages..."
              className="flex-1 rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm"
            />
            <button type="submit" className="rounded-lg bg-violet-500 px-4 py-2 text-sm font-semibold">
              Search
            </button>
          </form>
          {results && (
            <ul className="mt-4 space-y-2 text-sm text-gray-300">
              {results.length === 0 && <li className="text-gray-500">No results.</li>}
              {results.map((m) => (
                <li key={m.id} className="rounded-lg border border-white/10 p-3">
                  {m.text}
                </li>
              ))}
            </ul>
          )}
        </section>

        <section>
          <h2 className="text-xl font-semibold">Action items</h2>
          <ul className="mt-3 space-y-2">
            {actionItems.map((item) => (
              <li
                key={item.id}
                className="flex items-center justify-between rounded-lg border border-white/10 p-3 text-sm"
              >
                <span className={item.status === 'done' ? 'text-gray-500 line-through' : ''}>{item.task}</span>
                <button
                  onClick={() => handleToggleDone(item)}
                  className="rounded-full border border-white/10 px-3 py-1 text-xs"
                >
                  {item.status === 'done' ? 'Reopen' : 'Mark done'}
                </button>
              </li>
            ))}
            {actionItems.length === 0 && <li className="text-gray-500">No action items.</li>}
          </ul>
        </section>

        <section>
          <h2 className="text-xl font-semibold">Daily summaries</h2>
          <ul className="mt-3 space-y-4">
            {summaries.map((s) => (
              <li key={s.summary_date_utc} className="rounded-lg border border-white/10 p-4">
                <p className="text-sm font-semibold text-violet-300">{s.summary_date_utc}</p>
                <p className="mt-2 text-sm text-gray-300">{s.summary}</p>
              </li>
            ))}
            {summaries.length === 0 && <li className="text-gray-500">No summaries yet.</li>}
          </ul>
        </section>
      </div>
    </div>
  )
}
