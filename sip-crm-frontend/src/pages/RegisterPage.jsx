import { useState } from 'react'
import { auth, setToken } from '../api'
import { Headphones, AlertCircle } from 'lucide-react'

export default function RegisterPage({ onLogin }) {
  // onLogin receives { token, agentId, username, displayName, extension }
  const [form, setForm] = useState({ username: '', password: '', displayName: '', extension: '' })
  const [disclaimerAccepted, setDisclaimerAccepted] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleRegister = async (e) => {
    e.preventDefault()
    if (!disclaimerAccepted) {
      setError('You must accept the disclaimer and site policy to register.')
      return
    }
    setLoading(true)
    setError('')
    try {
      const res = await auth.register(form)
      setToken(res.token)
      onLogin(res)
    } catch (err) {
      setError(err.message)
    }
    setLoading(false)
  }

  return (
    <div className="min-h-screen bg-slate-950 text-white flex items-center justify-center p-4">
      <div className="w-full max-w-lg">
        <div className="text-center mb-8">
          <div className="w-14 h-14 bg-gradient-to-br from-blue-500 to-cyan-400 rounded-2xl flex items-center justify-center mx-auto mb-4">
            <Headphones className="w-7 h-7 text-white" />
          </div>
          <h1 className="text-2xl font-bold">Create Your Account</h1>
          <p className="text-slate-400 mt-1">Start your free trial today</p>
        </div>

        {error && <div className="bg-red-500/10 border border-red-500/20 text-red-400 p-3 rounded-lg mb-4 text-sm">{error}</div>}

        <form onSubmit={handleRegister} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Display Name</label>
            <input type="text" value={form.displayName} onChange={e => setForm({ ...form, displayName: e.target.value })}
              className="w-full px-4 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:ring-2 focus:ring-blue-500 outline-none" required placeholder="John Doe" />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Username</label>
            <input type="text" value={form.username} onChange={e => setForm({ ...form, username: e.target.value })}
              className="w-full px-4 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:ring-2 focus:ring-blue-500 outline-none" required placeholder="johndoe" />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Password</label>
            <input type="password" value={form.password} onChange={e => setForm({ ...form, password: e.target.value })}
              className="w-full px-4 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:ring-2 focus:ring-blue-500 outline-none" required placeholder="Min 6 characters" />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">SIP Extension</label>
            <input type="text" value={form.extension} onChange={e => setForm({ ...form, extension: e.target.value })}
              className="w-full px-4 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:ring-2 focus:ring-blue-500 outline-none" required placeholder="5001" />
          </div>

          <div className="bg-slate-800/50 border border-slate-700 rounded-xl p-4 mt-4">
            <h3 className="text-sm font-bold text-amber-400 flex items-center gap-2 mb-2">
              <AlertCircle className="w-4 h-4" /> Disclaimer & Site Policy
            </h3>
            <div className="text-xs text-slate-400 space-y-2 max-h-40 overflow-y-auto pr-2">
              <p><strong className="text-slate-300">Legal Compliance:</strong> By using this service, you agree to comply with all applicable laws and regulations in your country of residence regarding telecommunications, marketing calls, and electronic communications.</p>
              <p><strong className="text-slate-300">Illegal Calls:</strong> This platform strictly prohibits the use of its services for any illegal or unauthorized calls, including but not limited to: robocalls in violation of regulations, unsolicited marketing calls to numbers on Do Not Call registries, fraud or scam-related calls, calls that violate local telecommunications laws, and any calls that infringe upon the privacy rights of individuals.</p>
              <p><strong className="text-slate-300">User Responsibility:</strong> The user bears sole responsibility for ensuring that all calls made through this service comply with local, national, and international laws. <strong className="text-amber-400">CallFlow is not responsible for any illegal calls made by users.</strong> We cooperate fully with law enforcement agencies in investigations of illegal activity.</p>
              <p><strong className="text-slate-300">Account Termination:</strong> Accounts found to be in violation of this policy will be terminated immediately without refund. We reserve the right to report violations to relevant authorities.</p>
              <p><strong className="text-slate-300">Exchange Recommendations:</strong> Any market recommendations, target prices, or stop-loss signals provided are for informational purposes only and do not constitute financial advice. Users should consult qualified financial advisors before making investment decisions.</p>
            </div>
            <label className="flex items-start gap-3 mt-3 cursor-pointer">
              <input type="checkbox" checked={disclaimerAccepted} onChange={e => setDisclaimerAccepted(e.target.checked)}
                className="mt-0.5 w-4 h-4 rounded border-slate-600 bg-slate-700 text-blue-500 focus:ring-blue-500" />
              <span className="text-xs text-slate-300">I have read and agree to the Disclaimer & Site Policy. I understand that I am solely responsible for ensuring my use complies with all applicable laws, and that CallFlow is not liable for any illegal calls I may make.</span>
            </label>
          </div>

          <button type="submit" disabled={loading || !disclaimerAccepted}
            className="w-full py-3 bg-gradient-to-r from-blue-600 to-cyan-500 text-white font-bold rounded-xl hover:from-blue-500 hover:to-cyan-400 disabled:opacity-40 disabled:cursor-not-allowed transition shadow-lg shadow-blue-500/25">
            {loading ? 'Creating Account...' : 'Create Account'}
          </button>
        </form>

        <p className="text-center text-slate-500 text-sm mt-6">
          Already have an account? <button onClick={onLogin} className="text-blue-400 hover:text-blue-300 font-medium">Sign In</button>
        </p>
      </div>
    </div>
  )
}
