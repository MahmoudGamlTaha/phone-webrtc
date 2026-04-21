import { useState, useEffect } from 'react'
import { auth, clearToken, getToken } from './api'
import * as sip from './sip'
import LandingPage from './pages/LandingPage'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import DashboardPage from './pages/DashboardPage'

export default function App() {
  const [page, setPage] = useState('landing')
  const [agent, setAgent] = useState(null)

  useEffect(() => {
    const token = getToken()
    if (token) {
      auth.profile().then(a => {
        setAgent({ id: a.id, username: a.username, displayName: a.displayName, extension: a.extension })
        setPage('dashboard')
      }).catch(() => { clearToken(); setAgent(null) })
    }
  }, [])

  const handleLogin = (res) => {
    setAgent({ id: res.agentId, username: res.username, displayName: res.displayName, extension: res.extension })
    setPage('dashboard')
  }

  const handleLogout = () => {
    sip.disconnect()
    clearToken()
    setAgent(null)
    setPage('landing')
  }

  switch (page) {
    case 'landing':
      return <LandingPage onLogin={() => setPage('login')} onRegister={() => setPage('register')} />
    case 'login':
      return <LoginPage onLogin={handleLogin} onRegister={() => setPage('register')} onBack={() => setPage('landing')} />
    case 'register':
      return <RegisterPage onLogin={() => setPage('login')} />
    case 'dashboard':
      return <DashboardPage agent={agent} onLogout={handleLogout} />
    default:
      return <LandingPage onLogin={() => setPage('login')} onRegister={() => setPage('register')} />
  }
}
