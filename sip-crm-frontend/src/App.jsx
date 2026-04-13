import { useState, useEffect, useCallback } from 'react'
import { auth, customers, calls, tasks, dashboard, setToken, clearToken, getToken } from './api'
import * as sip from './sip'
import { Phone, PhoneOff, Users, ListTodo, BarChart3, LogOut, Plus, X, User, Clock, CheckCircle, AlertCircle, PhoneCall, Settings } from 'lucide-react'

// ─── Login Page ───
function LoginPage({ onLogin }) {
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
    <div className="min-h-screen bg-gradient-to-br from-slate-900 to-slate-800 flex items-center justify-center p-4">
      <div className="bg-white rounded-2xl shadow-2xl p-8 w-full max-w-md">
        <div className="text-center mb-8">
          <div className="w-16 h-16 bg-blue-600 rounded-2xl flex items-center justify-center mx-auto mb-4">
            <PhoneCall className="w-8 h-8 text-white" />
          </div>
          <h1 className="text-2xl font-bold text-gray-900">SIP CRM</h1>
          <p className="text-gray-500 mt-1">Agent Login</p>
        </div>
        {error && <div className="bg-red-50 text-red-600 p-3 rounded-lg mb-4 text-sm">{error}</div>}
        <form onSubmit={handleLogin}>
          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 mb-1">Username</label>
            <input type="text" value={username} onChange={e => setUsername(e.target.value)}
              className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none" required />
          </div>
          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
            <input type="password" value={password} onChange={e => setPassword(e.target.value)}
              className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none" required />
          </div>
          <button type="submit" disabled={loading}
            className="w-full bg-blue-600 text-white py-2.5 rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50 transition">
            {loading ? 'Signing in...' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  )
}

// ─── Dialer Component ───
function Dialer({ extension, callStatus, onDial, onHangup }) {
  const [number, setNumber] = useState('')

  const handleDial = () => {
    if (number.trim()) {
      onDial(number.trim())
    }
  }

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') handleDial()
  }

  return (
    <div className="bg-white rounded-xl shadow-sm border p-4">
      <h3 className="font-semibold text-gray-900 mb-3 flex items-center gap-2">
        <Phone className="w-4 h-4" /> Dial Pad
      </h3>
      <div className="flex gap-2">
        <input type="text" value={number} onChange={e => setNumber(e.target.value)}
          onKeyDown={handleKeyDown} placeholder="Extension number..."
          disabled={callStatus === 'dialing' || callStatus === 'connected'}
          className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none disabled:bg-gray-100" />
        {callStatus === 'dialing' || callStatus === 'connected' ? (
          <button onClick={onHangup}
            className="px-4 py-2 bg-red-600 text-white rounded-lg text-sm font-medium hover:bg-red-700 flex items-center gap-1">
            <PhoneOff className="w-4 h-4" /> Hangup
          </button>
        ) : (
          <button onClick={handleDial} disabled={!number.trim()}
            className="px-4 py-2 bg-green-600 text-white rounded-lg text-sm font-medium hover:bg-green-700 disabled:opacity-50 flex items-center gap-1">
            <Phone className="w-4 h-4" /> Call
          </button>
        )}
      </div>
      {callStatus === 'dialing' && (
        <div className="mt-2 flex items-center gap-2 text-amber-600 text-sm">
          <div className="w-2 h-2 bg-amber-500 rounded-full animate-pulse" /> Dialing {extension}...
        </div>
      )}
      {callStatus === 'connected' && (
        <div className="mt-2 flex items-center gap-2 text-green-600 text-sm">
          <CheckCircle className="w-4 h-4" /> Connected to {extension}
        </div>
      )}
      <audio id="remote-audio" autoPlay />
    </div>
  )
}

// ─── Customer List ───
function CustomerList({ customers: customerData, onCallCustomer, onEditCustomer }) {
  return (
    <div className="bg-white rounded-xl shadow-sm border">
      <div className="p-4 border-b flex items-center justify-between">
        <h3 className="font-semibold text-gray-900 flex items-center gap-2">
          <Users className="w-4 h-4" /> Customers ({customerData.length})
        </h3>
        <button onClick={() => onEditCustomer(null)}
          className="px-3 py-1.5 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 flex items-center gap-1">
          <Plus className="w-3 h-3" /> Add
        </button>
      </div>
      <div className="divide-y max-h-96 overflow-y-auto">
        {customerData.length === 0 && <div className="p-4 text-gray-400 text-center text-sm">No customers yet</div>}
        {customerData.map(c => (
          <div key={c.id} className="p-3 hover:bg-gray-50 flex items-center justify-between">
            <div className="flex-1 min-w-0">
              <div className="font-medium text-gray-900 truncate">{c.name}</div>
              <div className="text-sm text-gray-500 flex items-center gap-3">
                <span className="flex items-center gap-1"><Phone className="w-3 h-3" />{c.phone}</span>
                {c.company && <span>{c.company}</span>}
                <span className="flex items-center gap-1"><PhoneCall className="w-3 h-3" />{c.callCount} calls</span>
              </div>
            </div>
            <div className="flex items-center gap-1">
              <button onClick={() => onCallCustomer(c)} title="Call"
                className="p-2 text-green-600 hover:bg-green-50 rounded-lg"><Phone className="w-4 h-4" /></button>
              <button onClick={() => onEditCustomer(c)} title="Edit"
                className="p-2 text-blue-600 hover:bg-blue-50 rounded-lg"><User className="w-4 h-4" /></button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── Customer Modal ───
function CustomerModal({ customer, onClose, onSave }) {
  const [form, setForm] = useState(customer || { name: '', phone: '', email: '', company: '', notes: '' })
  const [saving, setSaving] = useState(false)

  const handleSave = async () => {
    setSaving(true)
    try {
      if (customer?.id) {
        await customers.update({ ...form, id: customer.id })
      } else {
        await customers.create(form)
      }
      onSave()
      onClose()
    } catch (err) {
      alert(err.message)
    }
    setSaving(false)
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-xl max-w-md w-full p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold text-lg">{customer?.id ? 'Edit Customer' : 'New Customer'}</h3>
          <button onClick={onClose} className="p-1 hover:bg-gray-100 rounded"><X className="w-5 h-5" /></button>
        </div>
        <div className="space-y-3">
          {[
            { key: 'name', label: 'Name' },
            { key: 'phone', label: 'Phone / Extension' },
            { key: 'email', label: 'Email' },
            { key: 'company', label: 'Company' },
          ].map(f => (
            <div key={f.key}>
              <label className="block text-sm font-medium text-gray-700 mb-1">{f.label}</label>
              <input type="text" value={form[f.key]} onChange={e => setForm({ ...form, [f.key]: e.target.value })}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none" />
            </div>
          ))}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Notes</label>
            <textarea value={form.notes} onChange={e => setForm({ ...form, notes: e.target.value })} rows={2}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none" />
          </div>
        </div>
        <div className="flex justify-end gap-2 mt-4">
          <button onClick={onClose} className="px-4 py-2 text-gray-600 hover:bg-gray-100 rounded-lg text-sm">Cancel</button>
          <button onClick={handleSave} disabled={saving || !form.name}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
            {saving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── Call History ───
function CallHistory({ calls: callData }) {
  const statusIcon = (s) => {
    if (s === 'answered') return <CheckCircle className="w-4 h-4 text-green-500" />
    if (s === 'no-answer') return <AlertCircle className="w-4 h-4 text-amber-500" />
    return <AlertCircle className="w-4 h-4 text-red-500" />
  }
  return (
    <div className="bg-white rounded-xl shadow-sm border">
      <div className="p-4 border-b">
        <h3 className="font-semibold text-gray-900 flex items-center gap-2">
          <Clock className="w-4 h-4" /> Call History
        </h3>
      </div>
      <div className="divide-y max-h-64 overflow-y-auto">
        {callData.length === 0 && <div className="p-4 text-gray-400 text-center text-sm">No calls yet</div>}
        {callData.map(c => (
          <div key={c.id} className="p-3 flex items-center gap-3 text-sm">
            {statusIcon(c.status)}
            <div className="flex-1 min-w-0">
              <span className="font-medium">{c.extension}</span>
              {c.customerName && <span className="text-gray-500 ml-1">- {c.customerName}</span>}
              <div className="text-gray-400 text-xs">{c.startedAt} &middot; {c.duration}s</div>
            </div>
            <span className={`px-2 py-0.5 rounded text-xs font-medium ${c.direction === 'outbound' ? 'bg-blue-50 text-blue-600' : 'bg-purple-50 text-purple-600'}`}>
              {c.direction}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── Task List ───
function TaskList({ tasks: taskData, onAddTask, onToggleTask }) {
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ title: '', description: '', dueDate: new Date().toISOString().slice(0, 10) })

  const handleAdd = async () => {
    try {
      await tasks.create({ ...form, status: 'pending' })
      onAddTask()
      setShowForm(false)
      setForm({ title: '', description: '', dueDate: new Date().toISOString().slice(0, 10) })
    } catch (err) {
      alert(err.message)
    }
  }

  return (
    <div className="bg-white rounded-xl shadow-sm border">
      <div className="p-4 border-b flex items-center justify-between">
        <h3 className="font-semibold text-gray-900 flex items-center gap-2">
          <ListTodo className="w-4 h-4" /> Today's Tasks
        </h3>
        <button onClick={() => setShowForm(true)}
          className="px-3 py-1.5 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 flex items-center gap-1">
          <Plus className="w-3 h-3" /> Add
        </button>
      </div>
      {showForm && (
        <div className="p-3 border-b bg-gray-50 space-y-2">
          <input type="text" placeholder="Task title" value={form.title}
            onChange={e => setForm({ ...form, title: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none" />
          <input type="date" value={form.dueDate} onChange={e => setForm({ ...form, dueDate: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none" />
          <div className="flex gap-2">
            <button onClick={handleAdd} disabled={!form.title}
              className="px-3 py-1.5 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 disabled:opacity-50">Save</button>
            <button onClick={() => setShowForm(false)}
              className="px-3 py-1.5 text-gray-600 hover:bg-gray-100 rounded-lg text-sm">Cancel</button>
          </div>
        </div>
      )}
      <div className="divide-y max-h-64 overflow-y-auto">
        {taskData.length === 0 && <div className="p-4 text-gray-400 text-center text-sm">No tasks</div>}
        {taskData.map(t => (
          <div key={t.id} className="p-3 flex items-center gap-3">
            <button onClick={() => onToggleTask(t)}
              className={`flex-shrink-0 w-5 h-5 rounded border-2 flex items-center justify-center ${t.status === 'done' ? 'bg-green-500 border-green-500' : 'border-gray-300 hover:border-blue-400'}`}>
              {t.status === 'done' && <CheckCircle className="w-3 h-3 text-white" />}
            </button>
            <div className="flex-1 min-w-0">
              <div className={`text-sm ${t.status === 'done' ? 'line-through text-gray-400' : 'text-gray-900'}`}>{t.title}</div>
              <div className="text-xs text-gray-400">Due: {t.dueDate}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── Dashboard Stats ───
function DashboardStats({ stats }) {
  return (
    <div className="grid grid-cols-3 gap-3">
      {[
        { label: 'Total Calls', value: stats.totalCalls || 0, icon: PhoneCall, color: 'blue' },
        { label: 'Answered', value: stats.answeredCalls || 0, icon: CheckCircle, color: 'green' },
        { label: 'Pending Tasks', value: stats.pendingTasks || 0, icon: ListTodo, color: 'amber' },
      ].map(s => (
        <div key={s.label} className="bg-white rounded-xl shadow-sm border p-4">
          <div className="flex items-center gap-2 mb-1">
            <s.icon className={`w-4 h-4 text-${s.color}-500`} />
            <span className="text-sm text-gray-500">{s.label}</span>
          </div>
          <div className="text-2xl font-bold text-gray-900">{s.value}</div>
        </div>
      ))}
    </div>
  )
}

// ─── Main Dashboard ───
function Dashboard({ agent, onLogout }) {
  const [callStatus, setCallStatus] = useState('idle') // idle, dialing, connected
  const [currentExt, setCurrentExt] = useState('')
  const [customerList, setCustomerList] = useState([])
  const [callHistory, setCallHistory] = useState([])
  const [taskList, setTaskList] = useState([])
  const [stats, setStats] = useState({})
  const [editingCustomer, setEditingCustomer] = useState(null)
  const [wsConnected, setWsConnected] = useState(false)

  const loadData = useCallback(async () => {
    try {
      const [c, h, t, s] = await Promise.all([
        customers.list(),
        calls.list(20),
        tasks.list('pending'),
        dashboard.stats(),
      ])
      setCustomerList(c || [])
      setCallHistory(h || [])
      setTaskList(t || [])
      setStats(s || {})
    } catch (err) {
      console.error('Load data error:', err)
    }
  }, [])

  useEffect(() => {
    loadData()
    // Connect SIP WebSocket (idempotent - won't reconnect if already connected)
    sip.setEventHandler((event, data) => {
      if (event === 'ws-connected' || event === 'auth-ok') setWsConnected(true)
      if (event === 'ws-disconnected') setWsConnected(false)
      if (event === 'dialing') { setCallStatus('dialing'); setCurrentExt(data) }
      if (event === 'call-started') { setCallStatus('connected'); setCurrentExt(data); loadData() }
      if (event === 'call-ended' || event === 'dial-error') { setCallStatus('idle'); setCurrentExt(''); loadData() }
    })
    sip.connect()
  }, [loadData])

  const handleCallCustomer = (customer) => {
    const ext = customer.phone || ''
    if (ext) {
      sip.dial(ext, customer.id)
      setCallStatus('dialing')
      setCurrentExt(ext)
    }
  }

  const handleDial = (ext) => {
    sip.dial(ext)
    setCallStatus('dialing')
    setCurrentExt(ext)
  }

  const handleHangup = () => {
    sip.hangup()
    setCallStatus('idle')
    setCurrentExt('')
  }

  const handleToggleTask = async (task) => {
    try {
      await tasks.update({ ...task, status: task.status === 'done' ? 'pending' : 'done' })
      loadData()
    } catch (err) {
      console.error(err)
    }
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white border-b sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center">
              <PhoneCall className="w-4 h-4 text-white" />
            </div>
            <h1 className="text-lg font-bold text-gray-900">SIP CRM</h1>
            <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${wsConnected ? 'bg-green-50 text-green-600' : 'bg-red-50 text-red-600'}`}>
              {wsConnected ? 'Online' : 'Offline'}
            </span>
          </div>
          <div className="flex items-center gap-4">
            <div className="text-right">
              <div className="text-sm font-medium text-gray-900">{agent.displayName}</div>
              <div className="text-xs text-gray-500">Ext: {agent.extension}</div>
            </div>
            <button onClick={onLogout} className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg" title="Logout">
              <LogOut className="w-5 h-5" />
            </button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 py-6 space-y-4">
        <DashboardStats stats={stats} />

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          {/* Left: Dialer + Call History */}
          <div className="space-y-4">
            <Dialer extension={currentExt} callStatus={callStatus} onDial={handleDial} onHangup={handleHangup} />
            <CallHistory calls={callHistory} />
          </div>

          {/* Center: Customers */}
          <div>
            <CustomerList customers={customerList} onCallCustomer={handleCallCustomer} onEditCustomer={setEditingCustomer} />
          </div>

          {/* Right: Tasks */}
          <div>
            <TaskList tasks={taskList} onAddTask={loadData} onToggleTask={handleToggleTask} />
          </div>
        </div>
      </main>

      {/* Customer Modal */}
      {editingCustomer !== undefined && editingCustomer !== null && (
        <CustomerModal customer={editingCustomer} onClose={() => setEditingCustomer(undefined)} onSave={loadData} />
      )}
      {editingCustomer === null && (
        <CustomerModal customer={null} onClose={() => setEditingCustomer(undefined)} onSave={loadData} />
      )}
    </div>
  )
}

// ─── App Root ───
export default function App() {
  const [agent, setAgent] = useState(null)

  useEffect(() => {
    const token = getToken()
    if (token) {
      auth.profile().then(setAgent).catch(() => { clearToken(); setAgent(null) })
    }
  }, [])

  const handleLogin = (res) => {
    setAgent({ id: res.agentId, username: res.username, displayName: res.displayName, extension: res.extension })
  }

  const handleLogout = () => {
    sip.disconnect()
    clearToken()
    setAgent(null)
  }

  if (!agent) return <LoginPage onLogin={handleLogin} />
  return <Dashboard agent={agent} onLogout={handleLogout} />
}
