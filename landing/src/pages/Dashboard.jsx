import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { listChats } from '../lib/api'

export default function Dashboard() {
  const [chats, setChats] = useState([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    listChats()
      .then(setChats)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="min-h-screen bg-[#15131f] px-6 py-12 text-white">
      <div className="mx-auto max-w-2xl">
        <h1 className="text-2xl font-semibold">Your chats</h1>
        {loading && <p className="mt-6 text-gray-400">Loading...</p>}
        {error && <p className="mt-6 text-red-400">{error}</p>}
        {!loading && !error && chats.length === 0 && (
          <p className="mt-6 text-gray-400">
            No chats yet. Send <code className="rounded bg-white/10 px-1.5 py-0.5">/dashboard</code> in a
            Telegram group with ChatVault to get a link.
          </p>
        )}
        <ul className="mt-6 divide-y divide-white/10">
          {chats.map((chat) => (
            <li key={chat.chat_id}>
              <Link
                to={`/dashboard/chats/${chat.chat_id}`}
                className="flex items-center justify-between py-4 transition hover:text-violet-300"
              >
                <span>{chat.chat_title || `Chat ${chat.chat_id}`}</span>
                <span className="text-xs text-gray-500">{chat.role}</span>
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
