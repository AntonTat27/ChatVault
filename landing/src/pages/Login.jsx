import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

import TelegramLoginButton from '../components/TelegramLoginButton'
import { telegramLogin } from '../lib/api'

const DEV_AUTH_BYPASS = import.meta.env.VITE_DEV_AUTH_BYPASS === 'true'

export default function Login() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [error, setError] = useState('')
  const [devId, setDevId] = useState('12345678')
  const [devName, setDevName] = useState('Dev User')
  const chatId = searchParams.get('chat_id')

  async function handleAuth(user) {
    setError('')
    try {
      await telegramLogin(user)
      navigate(chatId ? `/dashboard/chats/${chatId}` : '/dashboard')
    } catch (err) {
      setError(err.message || 'Login failed.')
    }
  }

  async function handleDevLogin(e) {
    e.preventDefault()
    await handleAuth({ id: devId, first_name: devName, auth_date: '1000000000', hash: 'dev' })
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-6 bg-[#15131f] px-6 text-center">
      <h1 className="text-2xl font-semibold text-white">Sign in to ChatVault</h1>
      <p className="max-w-sm text-sm text-gray-400">
        Use your Telegram account to access your chat's dashboard.
      </p>
      {DEV_AUTH_BYPASS ? (
        <form onSubmit={handleDevLogin} className="flex flex-col gap-3 w-full max-w-xs">
          <p className="text-xs text-yellow-400 border border-yellow-600 rounded px-3 py-1">
            Dev bypass — auth checks disabled
          </p>
          <input
            value={devId}
            onChange={e => setDevId(e.target.value)}
            placeholder="Telegram User ID"
            className="rounded bg-[#2a2840] px-3 py-2 text-white text-sm outline-none"
          />
          <input
            value={devName}
            onChange={e => setDevName(e.target.value)}
            placeholder="First Name"
            className="rounded bg-[#2a2840] px-3 py-2 text-white text-sm outline-none"
          />
          <button type="submit" className="rounded bg-[#229ed9] px-4 py-2 text-white text-sm font-medium hover:opacity-90">
            Login as Dev User
          </button>
        </form>
      ) : (
        <TelegramLoginButton onAuth={handleAuth} />
      )}
      {error && <p className="text-sm text-red-400">{error}</p>}
    </div>
  )
}
