import { useEffect } from 'react'
import { Route, Routes, useNavigate } from 'react-router-dom'

import Actions from './pages/Actions'
import ChatDetail from './pages/ChatDetail'
import Dashboard from './pages/Dashboard'
import Decisions from './pages/Decisions'
import Home from './pages/Home'
import Ideas from './pages/Ideas'
import Integrations from './pages/Integrations'
import Login from './pages/Login'

function App() {
  const navigate = useNavigate()

  useEffect(() => {
    const redirect = sessionStorage.redirect
    if (redirect) {
      delete sessionStorage.redirect
      navigate(redirect)
    }
  }, [navigate])

  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/login" element={<Login />} />
      <Route path="/dashboard" element={<Dashboard />} />
      <Route path="/dashboard/chats/:chatId" element={<ChatDetail />} />
      <Route path="/dashboard/chats/:chatId/integrations" element={<Integrations />} />
      <Route path="/dashboard/chats/:chatId/decisions" element={<Decisions />} />
      <Route path="/dashboard/chats/:chatId/actions" element={<Actions />} />
      <Route path="/dashboard/chats/:chatId/ideas" element={<Ideas />} />
    </Routes>
  )
}

export default App
