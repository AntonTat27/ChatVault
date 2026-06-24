import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

import TelegramLoginButton from '../components/TelegramLoginButton'
import { telegramLogin } from '../lib/api'

export default function Login() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [error, setError] = useState('')
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

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-6 bg-[#15131f] px-6 text-center">
      <h1 className="text-2xl font-semibold text-white">Sign in to ChatVault</h1>
      <p className="max-w-sm text-sm text-gray-400">
        Use your Telegram account to access your chat's dashboard.
      </p>
      <TelegramLoginButton onAuth={handleAuth} />
      {error && <p className="text-sm text-red-400">{error}</p>}
    </div>
  )
}
