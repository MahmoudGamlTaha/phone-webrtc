import { useState } from 'react'
import { auth, setToken } from '../api'
import { Headphones, ArrowRight } from 'lucide-react'

export default function LoginPage({ onLogin, onRegister, onBack }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleLogin = async (e) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const res = await auth.login(username, password)
      setToken(res.token)
      onLogin(res)
    } catch (err) {
      setError(err.message)
    }
    setLoading(false)
  }

  return (
    <div className="min-h-screen bg-slate-950 text-white flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <div className="w-14 h-14 bg-gradient-to-br from-blue-500 to-cyan-400 rounded-2xl flex items-center justify-center mx-auto mb-4">
            <Headphones className="w-7 h-7 text-white" />
          </div>
          <h1 className="text-2xl font-bold">Welcome Back</h1>
          <p className="text-slate-400 mt-1">Sign in to your CallFlow account</p>
        </div>
        {error && <div className="bg-red-500/10 border border-red-500/20 text-red-400 p-3 rounded-lg mb-4 text-sm">{error}</div>}
        <form onSubmit={handleLogin} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Username</label>
            <input type="text" value={username} onChange={e => setUsername(e.target.value)}
              className="w-full px-4 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:ring-2 focus:ring-blue-500 outline-none" required />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Password</label>
            <input type="password" value={password} onChange={e => setPassword(e.target.value)}
              className="w-full px-4 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:ring-2 focus:ring-blue-500 outline-none" required />
          </div>
          <button type="submit" disabled={loading}
            className="w-full py-3 bg-gradient-to-r from-blue-600 to-cyan-500 text-white font-bold rounded-xl hover:from-blue-500 hover:to-cyan-400 disabled:opacity-50 transition shadow-lg shadow-blue-500/25">
            {loading ? 'Signing in...' : 'Sign In'}
          </button>
        </form>
        <div className="flex items-center justify-between mt-6">
          <button onClick={onBack} className="text-slate-500 hover:text-slate-300 text-sm flex items-center gap-1">
            <ArrowRight className="w-4 h-4 rotate-180" /> Back to site
          </button>
          <button onClick={onRegister} className="text-blue-400 hover:text-blue-300 text-sm font-medium">Create account</button>
        </div>
      </div>
    </div>
  )
}
