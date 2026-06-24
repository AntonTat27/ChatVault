import { useEffect, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'

import { getNotionStatus, listNotionDatabases, notionOAuthStartHref, setNotionDatabase } from '../lib/api'

export default function Integrations() {
  const { chatId } = useParams()
  const [searchParams] = useSearchParams()
  const [status, setStatus] = useState(null)
  const [databases, setDatabases] = useState(null)
  const [error, setError] = useState('')
  const justConnected = searchParams.get('notion') === 'connected'

  useEffect(() => {
    getNotionStatus(chatId)
      .then(setStatus)
      .catch((err) => setError(err.message))
  }, [chatId])

  useEffect(() => {
    if (!justConnected) return
    listNotionDatabases(chatId)
      .then(setDatabases)
      .catch((err) => setError(err.message))
  }, [chatId, justConnected])

  async function handlePickDatabase(databaseId) {
    await setNotionDatabase(chatId, databaseId)
    setStatus((s) => ({ ...s, database_id: databaseId }))
  }

  return (
    <div className="min-h-screen bg-[#15131f] px-6 py-12 text-white">
      <div className="mx-auto max-w-2xl space-y-8">
        <h1 className="text-2xl font-semibold">Notion integration</h1>
        {error && <p className="text-red-400">{error}</p>}

        {status && !status.configured && (
          <div className="rounded-lg border border-white/10 p-6">
            <p className="text-sm text-gray-300">Connect a Notion workspace to export daily summaries.</p>
            <a
              href={notionOAuthStartHref(chatId)}
              className="mt-4 inline-flex rounded-full bg-violet-500 px-4 py-2 text-sm font-semibold"
            >
              Connect Notion
            </a>
          </div>
        )}

        {status && status.configured && (
          <div className="rounded-lg border border-white/10 p-6">
            <p className="text-sm text-gray-300">
              Connected to <span className="font-semibold text-white">{status.workspace_name || 'Notion'}</span>
            </p>
            <p className="mt-2 text-sm text-gray-400">
              Database: {status.database_id || 'not selected yet'}
            </p>
          </div>
        )}

        {databases && (
          <div className="rounded-lg border border-white/10 p-6">
            <h2 className="text-lg font-semibold">Choose a database</h2>
            <ul className="mt-4 space-y-2">
              {databases.length === 0 && <li className="text-gray-500">No databases found in this workspace.</li>}
              {databases.map((d) => (
                <li key={d.id}>
                  <button
                    onClick={() => handlePickDatabase(d.id)}
                    className={`w-full rounded-lg border px-4 py-3 text-left text-sm transition ${
                      status?.database_id === d.id
                        ? 'border-violet-500 bg-violet-500/10'
                        : 'border-white/10 hover:border-white/30'
                    }`}
                  >
                    {d.title}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </div>
  )
}
