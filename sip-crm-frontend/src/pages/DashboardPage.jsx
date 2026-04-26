import { useState, useEffect, useCallback } from 'react'
import { customers, calls, tasks, dashboard } from '../api'
import * as sip from '../sip'
import {
  Phone, PhoneOff, Users, ListTodo, LogOut, Plus, X, User, Clock,
  CheckCircle, AlertCircle, PhoneCall, Headphones, TrendingUp,
  Target, MinusCircle, LayoutDashboard
} from 'lucide-react'

function CustomerModal({ customer, onClose, onSave }) {
  const [form, setForm] = useState(customer || { name: '', phone: '', email: '', company: '', notes: '' })
  const [saving, setSaving] = useState(false)
  const handleSave = async () => {
    setSaving(true)
    try {
      if (customer?.id) await customers.update({ ...form, id: customer.id })
      else await customers.create(form)
      onSave(); onClose()
    } catch (err) { alert(err.message) }
    setSaving(false)
  }
  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-slate-900 border border-slate-700 rounded-2xl max-w-md w-full p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-bold">{customer?.id ? 'Edit Customer' : 'New Customer'}</h3>
          <button onClick={onClose} className="p-1 text-slate-400 hover:text-white rounded"><X className="w-5 h-5" /></button>
        </div>
        <div className="space-y-3">
          {[{ key: 'name', label: 'Name' }, { key: 'phone', label: 'Phone / Extension' }, { key: 'email', label: 'Email' }, { key: 'company', label: 'Company' }].map(f => (
            <div key={f.key}>
              <label className="block text-sm font-medium text-slate-300 mb-1">{f.label}</label>
              <input type="text" value={form[f.key]} onChange={e => setForm({ ...form, [f.key]: e.target.value })}
                className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-sm text-white focus:ring-2 focus:ring-blue-500 outline-none" />
            </div>
          ))}
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">Notes</label>
            <textarea value={form.notes} onChange={e => setForm({ ...form, notes: e.target.value })} rows={2}
              className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-sm text-white focus:ring-2 focus:ring-blue-500 outline-none" />
          </div>
        </div>
        <div className="flex justify-end gap-2 mt-4">
          <button onClick={onClose} className="px-4 py-2 text-slate-400 hover:bg-slate-800 rounded-lg text-sm">Cancel</button>
          <button onClick={handleSave} disabled={saving || !form.name}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50">{saving ? 'Saving...' : 'Save'}</button>
        </div>
      </div>
    </div>
  )
}

function TasksView({ taskList, onToggle, onRefresh }) {
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ title: '', description: '', dueDate: new Date().toISOString().slice(0, 10) })
  const handleAdd = async () => {
    try { await tasks.create({ ...form, status: 'pending' }); onRefresh(); setShowForm(false); setForm({ title: '', description: '', dueDate: new Date().toISOString().slice(0, 10) }) } catch (err) { alert(err.message) }
  }
  return (
    <>
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold">Tasks</h2>
        <button onClick={() => setShowForm(true)} className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 flex items-center gap-2"><Plus className="w-4 h-4" /> Add Task</button>
      </div>
      {showForm && (
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 space-y-3">
          <input type="text" placeholder="Task title" value={form.title} onChange={e => setForm({ ...form, title: e.target.value })}
            className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-sm text-white placeholder-slate-500 focus:ring-2 focus:ring-blue-500 outline-none" />
          <input type="date" value={form.dueDate} onChange={e => setForm({ ...form, dueDate: e.target.value })}
            className="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-sm text-white focus:ring-2 focus:ring-blue-500 outline-none" />
          <div className="flex gap-2">
            <button onClick={handleAdd} disabled={!form.title} className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 disabled:opacity-50">Save</button>
            <button onClick={() => setShowForm(false)} className="px-4 py-2 text-slate-400 hover:bg-slate-800 rounded-lg text-sm">Cancel</button>
          </div>
        </div>
      )}
      <div className="bg-slate-900 border border-slate-800 rounded-xl">
        <div className="divide-y divide-slate-800">
          {taskList.map(t => (
            <div key={t.id} className="p-4 flex items-center gap-3">
              <button onClick={() => onToggle(t)} className={`flex-shrink-0 w-5 h-5 rounded border-2 flex items-center justify-center ${t.status === 'done' ? 'bg-green-500 border-green-500' : 'border-slate-600 hover:border-blue-400'}`}>
                {t.status === 'done' && <CheckCircle className="w-3 h-3 text-white" />}
              </button>
              <div className="flex-1 min-w-0">
                <div className={`text-sm ${t.status === 'done' ? 'line-through text-slate-500' : 'text-white'}`}>{t.title}</div>
                <div className="text-xs text-slate-500">Due: {t.dueDate}</div>
              </div>
            </div>
          ))}
          {taskList.length === 0 && <div className="p-8 text-slate-500 text-center">No pending tasks</div>}
        </div>
      </div>
    </>
  )
}

export default function DashboardPage({ agent, onLogout }) {
  const [callStatus, setCallStatus] = useState('idle')
  const [currentExt, setCurrentExt] = useState('')
  const [dialError, setDialError] = useState('')
  const [customerList, setCustomerList] = useState([])
  const [callHistory, setCallHistory] = useState([])
  const [taskList, setTaskList] = useState([])
  const [stats, setStats] = useState({})
  const [editingCustomer, setEditingCustomer] = useState(null)
  const [wsConnected, setWsConnected] = useState(false)
  const [activeTab, setActiveTab] = useState('dashboard')
  const [dialNumber, setDialNumber] = useState('')

  const loadData = useCallback(async () => {
    try {
      const [c, h, t, s] = await Promise.all([customers.list(), calls.list(20), tasks.list('pending'), dashboard.stats()])
      setCustomerList(c || []); setCallHistory(h || []); setTaskList(t || []); setStats(s || {})
    } catch (err) { console.error('Load data error:', err) }
  }, [])

  useEffect(() => {
    loadData()
    sip.setEventHandler((event, data) => {
      if (event === 'ws-connected' || event === 'auth-ok') setWsConnected(true)
      if (event === 'ws-disconnected') setWsConnected(false)
      if (event === 'dialing') { setCallStatus('dialing'); setCurrentExt(data); setDialError('') }
      if (event === 'ringing') setCallStatus('ringing')
      if (event === 'call-started') { setCallStatus('connected'); setCurrentExt(data); setDialError(''); loadData() }
      if (event === 'call-ended') { setCallStatus('idle'); setCurrentExt(''); setDialError(''); loadData() }
      if (event === 'dial-error') { setCallStatus('idle'); setCurrentExt(''); setDialError(data || 'Call failed'); loadData() }
      if (event === 'mic-error') console.error('Mic access failed:', data, '- HTTPS may be required')
    })
    sip.connect()
  }, [loadData])

  const handleCallCustomer = (customer) => {
    const ext = customer.phone || ''
    if (ext && callStatus === 'idle') { sip.dial(ext, customer.id); setCallStatus('dialing'); setCurrentExt(ext) }
  }
  const handleDial = () => {
    if (dialNumber.trim() && callStatus === 'idle') { sip.dial(dialNumber.trim()); setCallStatus('dialing'); setCurrentExt(dialNumber.trim()) }
  }
  const handleHangup = () => { sip.hangup(); setCallStatus('idle'); setCurrentExt('') }
  const handleToggleTask = async (task) => {
    try { await tasks.update({ ...task, status: task.status === 'done' ? 'pending' : 'done' }); loadData() } catch (err) { console.error(err) }
  }

  const navItems = [
    { id: 'dashboard', icon: LayoutDashboard, label: 'Dashboard' },
    { id: 'customers', icon: Users, label: 'Customers' },
    { id: 'calls', icon: PhoneCall, label: 'Call Board' },
    { id: 'tasks', icon: ListTodo, label: 'Tasks' },
    { id: 'recommendations', icon: TrendingUp, label: 'Recommendations' },
  ]

  return (
    <div className="min-h-screen bg-slate-950 text-white flex">
      {/* Sidebar */}
      <aside className="w-64 bg-slate-900 border-r border-slate-800 flex flex-col flex-shrink-0">
        <div className="p-4 border-b border-slate-800">
          <div className="flex items-center gap-2">
            <div className="w-9 h-9 bg-gradient-to-br from-blue-500 to-cyan-400 rounded-xl flex items-center justify-center"><Headphones className="w-5 h-5 text-white" /></div>
            <span className="text-lg font-bold bg-gradient-to-r from-blue-400 to-cyan-300 bg-clip-text text-transparent">CallFlow</span>
          </div>
        </div>

        {/* Dialer */}
        <div className="p-4 border-b border-slate-800">
          <div className="flex gap-2">
            <input type="text" value={dialNumber} onChange={e => setDialNumber(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); handleDial() } }} placeholder="Dial extension..."
              disabled={callStatus !== 'idle'}
              className="flex-1 px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-sm text-white placeholder-slate-500 focus:ring-2 focus:ring-blue-500 outline-none disabled:opacity-50" />
            {callStatus === 'idle' ? (
              <button type="button" onClick={handleDial} disabled={!dialNumber.trim()} className="px-3 py-2 bg-green-600 text-white rounded-lg text-sm hover:bg-green-700 disabled:opacity-40"><Phone className="w-4 h-4" /></button>
            ) : (
              <button type="button" onClick={handleHangup} className="px-3 py-2 bg-red-600 text-white rounded-lg text-sm hover:bg-red-700"><PhoneOff className="w-4 h-4" /></button>
            )}
          </div>
          {callStatus === 'dialing' && <div className="mt-2 flex items-center justify-between gap-2 text-amber-400 text-xs"><div className="flex items-center gap-2"><div className="w-2 h-2 bg-amber-500 rounded-full animate-pulse" /> Dialing {currentExt}...</div><button type="button" onClick={handleHangup} className="px-2 py-1 bg-red-600 text-white rounded text-xs hover:bg-red-700">Cancel</button></div>}
          {callStatus === 'ringing' && <div className="mt-2 flex items-center justify-between gap-2 text-blue-400 text-xs"><div className="flex items-center gap-2"><Phone className="w-3 h-3 animate-pulse" /> Ringing {currentExt}...</div><button type="button" onClick={handleHangup} className="px-2 py-1 bg-red-600 text-white rounded text-xs hover:bg-red-700">Cancel</button></div>}
          {callStatus === 'connected' && <div className="mt-2 flex items-center justify-between gap-2 text-green-400 text-xs"><div className="flex items-center gap-2"><CheckCircle className="w-3 h-3" /> Connected to {currentExt}</div><button type="button" onClick={handleHangup} className="px-2 py-1 bg-red-600 text-white rounded text-xs hover:bg-red-700">Hang Up</button></div>}
          {dialError && <div className="mt-2 text-red-400 text-xs">{dialError}</div>}
          <div className="flex items-center gap-2 mt-2">
            <span className={`w-2 h-2 rounded-full ${wsConnected ? 'bg-green-500' : 'bg-red-500'}`} />
            <span className="text-xs text-slate-500">{wsConnected ? 'Online' : 'Offline'}</span>
          </div>
        </div>

        {/* Nav */}
        <nav className="flex-1 p-2 space-y-1">
          {navItems.map(item => (
            <button key={item.id} onClick={() => setActiveTab(item.id)}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition ${activeTab === item.id ? 'bg-blue-600/10 text-blue-400' : 'text-slate-400 hover:bg-slate-800 hover:text-white'}`}>
              <item.icon className="w-4 h-4" /> {item.label}
            </button>
          ))}
        </nav>

        {/* User */}
        <div className="p-4 border-t border-slate-800">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 bg-slate-700 rounded-full flex items-center justify-center text-sm font-bold text-slate-300">{agent.displayName?.charAt(0) || 'U'}</div>
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium text-white truncate">{agent.displayName}</div>
              <div className="text-xs text-slate-500">Ext: {agent.extension}</div>
            </div>
            <button onClick={onLogout} className="p-1.5 text-slate-500 hover:text-red-400 hover:bg-slate-800 rounded-lg" title="Logout"><LogOut className="w-4 h-4" /></button>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-y-auto">
        {activeTab === 'dashboard' && (
          <div className="p-6 space-y-6">
            <h2 className="text-2xl font-bold">Dashboard</h2>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              {[
                { label: 'Total Calls', value: stats.totalCalls || 0, icon: PhoneCall, color: 'blue' },
                { label: 'Answered', value: stats.answeredCalls || 0, icon: CheckCircle, color: 'green' },
                { label: 'Pending Tasks', value: stats.pendingTasks || 0, icon: ListTodo, color: 'amber' },
              ].map(s => (
                <div key={s.label} className="bg-slate-900 border border-slate-800 rounded-xl p-5">
                  <div className="flex items-center gap-2 mb-2"><s.icon className={`w-5 h-5 text-${s.color}-400`} /><span className="text-sm text-slate-400">{s.label}</span></div>
                  <div className="text-3xl font-bold">{s.value}</div>
                </div>
              ))}
            </div>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <div className="bg-slate-900 border border-slate-800 rounded-xl">
                <div className="p-4 border-b border-slate-800 flex items-center justify-between">
                  <h3 className="font-semibold flex items-center gap-2"><Clock className="w-4 h-4 text-blue-400" /> Recent Calls</h3>
                  <button onClick={() => setActiveTab('calls')} className="text-xs text-blue-400 hover:text-blue-300">View all</button>
                </div>
                <div className="divide-y divide-slate-800 max-h-64 overflow-y-auto">
                  {callHistory.slice(0, 5).map(c => (
                    <div key={c.id} className="p-3 flex items-center gap-3 text-sm">
                      {c.status === 'answered' ? <CheckCircle className="w-4 h-4 text-green-500" /> : <AlertCircle className="w-4 h-4 text-amber-500" />}
                      <div className="flex-1 min-w-0"><span className="font-medium">{c.extension}</span>{c.customerName && <span className="text-slate-500 ml-1">- {c.customerName}</span>}</div>
                      <span className="text-xs text-slate-500">{c.duration}s</span>
                    </div>
                  ))}
                  {callHistory.length === 0 && <div className="p-4 text-slate-500 text-center text-sm">No calls yet</div>}
                </div>
              </div>
              <div className="bg-slate-900 border border-slate-800 rounded-xl">
                <div className="p-4 border-b border-slate-800 flex items-center justify-between">
                  <h3 className="font-semibold flex items-center gap-2"><TrendingUp className="w-4 h-4 text-amber-400" /> Recommendations</h3>
                  <button onClick={() => setActiveTab('recommendations')} className="text-xs text-blue-400 hover:text-blue-300">View all</button>
                </div>
                <div className="divide-y divide-slate-800">
                  {[{ pair: 'EUR/USD', target: '1.0925', stop: '1.0845', dir: 'Buy' }, { pair: 'GBP/USD', target: '1.2780', stop: '1.2690', dir: 'Buy' }, { pair: 'XAU/USD', target: '2,435', stop: '2,380', dir: 'Sell' }].map((r, i) => (
                    <div key={i} className="p-3 flex items-center justify-between text-sm">
                      <span className="font-bold">{r.pair}</span>
                      <div className="flex items-center gap-4">
                        <span className="text-green-400 font-mono text-xs">T: {r.target}</span>
                        <span className="text-red-400 font-mono text-xs">SL: {r.stop}</span>
                        <span className={`text-xs font-bold px-2 py-0.5 rounded ${r.dir === 'Buy' ? 'bg-green-500/10 text-green-400' : 'bg-red-500/10 text-red-400'}`}>{r.dir}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'customers' && (
          <div className="p-6 space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-2xl font-bold">Customers</h2>
              <button onClick={() => setEditingCustomer(null)} className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 flex items-center gap-2"><Plus className="w-4 h-4" /> Add Customer</button>
            </div>
            <div className="bg-slate-900 border border-slate-800 rounded-xl">
              <div className="divide-y divide-slate-800">
                {customerList.map(c => (
                  <div key={c.id} className="p-4 flex items-center justify-between hover:bg-slate-800/50 transition">
                    <div className="flex-1 min-w-0">
                      <div className="font-medium">{c.name}</div>
                      <div className="text-sm text-slate-400 flex items-center gap-3">
                        <span className="flex items-center gap-1"><Phone className="w-3 h-3" />{c.phone}</span>
                        {c.company && <span>{c.company}</span>}
                        <span className="flex items-center gap-1"><PhoneCall className="w-3 h-3" />{c.callCount} calls</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <button type="button" onClick={() => handleCallCustomer(c)} className="p-2 text-green-400 hover:bg-green-500/10 rounded-lg" title="Call"><Phone className="w-4 h-4" /></button>
                      <button type="button" onClick={() => setEditingCustomer(c)} className="p-2 text-blue-400 hover:bg-blue-500/10 rounded-lg" title="Edit"><User className="w-4 h-4" /></button>
                    </div>
                  </div>
                ))}
                {customerList.length === 0 && <div className="p-8 text-slate-500 text-center">No customers yet</div>}
              </div>
            </div>
          </div>
        )}

        {activeTab === 'calls' && (
          <div className="p-6 space-y-4">
            <h2 className="text-2xl font-bold">Call Board</h2>
            <div className="bg-slate-900 border border-slate-800 rounded-xl">
              <div className="p-4 border-b border-slate-800"><h3 className="font-semibold flex items-center gap-2"><Clock className="w-4 h-4 text-blue-400" /> Call History</h3></div>
              <div className="divide-y divide-slate-800 max-h-[600px] overflow-y-auto">
                {callHistory.map(c => (
                  <div key={c.id} className="p-4 flex items-center gap-4">
                    {c.status === 'answered' ? <CheckCircle className="w-5 h-5 text-green-500" /> : c.status === 'no-answer' ? <AlertCircle className="w-5 h-5 text-amber-500" /> : <AlertCircle className="w-5 h-5 text-red-500" />}
                    <div className="flex-1 min-w-0"><div className="font-medium">{c.extension}</div>{c.customerName && <div className="text-sm text-slate-400">{c.customerName}</div>}</div>
                    <div className="text-right text-sm"><div className="text-slate-400">{c.startedAt}</div><div className="text-slate-500">{c.duration}s</div></div>
                    <span className={`px-2 py-1 rounded text-xs font-bold ${c.direction === 'outbound' ? 'bg-blue-500/10 text-blue-400' : 'bg-purple-500/10 text-purple-400'}`}>{c.direction}</span>
                  </div>
                ))}
                {callHistory.length === 0 && <div className="p-8 text-slate-500 text-center">No calls yet</div>}
              </div>
            </div>
          </div>
        )}

        {activeTab === 'tasks' && (
          <div className="p-6 space-y-4">
            <TasksView taskList={taskList} onToggle={handleToggleTask} onRefresh={loadData} />
          </div>
        )}

        {activeTab === 'recommendations' && (
          <div className="p-6 space-y-6">
            <h2 className="text-2xl font-bold flex items-center gap-2"><TrendingUp className="w-6 h-6 text-amber-400" /> Exchange Recommendations</h2>
            <p className="text-slate-400 text-sm">Real-time target price and stop-loss signals. For informational purposes only — not financial advice.</p>
            <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {[
                { pair: 'EUR/USD', target: '1.0925', stop: '1.0845', dir: 'Buy', change: '+0.35%', confidence: 'High' },
                { pair: 'GBP/USD', target: '1.2780', stop: '1.2690', dir: 'Buy', change: '+0.22%', confidence: 'Medium' },
                { pair: 'USD/JPY', target: '155.20', stop: '154.40', dir: 'Sell', change: '-0.28%', confidence: 'High' },
                { pair: 'XAU/USD', target: '2,435', stop: '2,380', dir: 'Sell', change: '-0.15%', confidence: 'Medium' },
                { pair: 'USD/CHF', target: '0.9120', stop: '0.9050', dir: 'Buy', change: '+0.18%', confidence: 'Low' },
                { pair: 'AUD/USD', target: '0.6625', stop: '0.6560', dir: 'Sell', change: '-0.12%', confidence: 'Medium' },
              ].map((r, i) => (
                <div key={i} className="bg-slate-900 border border-slate-800 rounded-xl p-5">
                  <div className="flex items-center justify-between mb-4">
                    <span className="text-lg font-bold">{r.pair}</span>
                    <span className={`text-xs font-bold px-2.5 py-1 rounded-lg ${r.dir === 'Buy' ? 'bg-green-500/10 text-green-400' : 'bg-red-500/10 text-red-400'}`}>{r.dir}</span>
                  </div>
                  <div className="space-y-3 text-sm">
                    <div className="flex items-center justify-between"><span className="text-slate-400 flex items-center gap-1"><Target className="w-3 h-3" /> Target Price</span><span className="text-green-400 font-mono font-bold">{r.target}</span></div>
                    <div className="flex items-center justify-between"><span className="text-slate-400 flex items-center gap-1"><MinusCircle className="w-3 h-3" /> Stop Loss</span><span className="text-red-400 font-mono font-bold">{r.stop}</span></div>
                    <div className="flex items-center justify-between"><span className="text-slate-400">Change</span><span className={r.change.startsWith('+') ? 'text-green-400' : 'text-red-400'}>{r.change}</span></div>
                    <div className="flex items-center justify-between"><span className="text-slate-400">Confidence</span><span className={`font-medium ${r.confidence === 'High' ? 'text-green-400' : r.confidence === 'Medium' ? 'text-amber-400' : 'text-slate-400'}`}>{r.confidence}</span></div>
                  </div>
                </div>
              ))}
            </div>
            <p className="text-slate-600 text-xs text-center">Recommendations are for informational purposes only and do not constitute financial advice.</p>
          </div>
        )}
      </main>

      {/* Customer Modal */}
      {(editingCustomer !== undefined && editingCustomer !== null) && <CustomerModal customer={editingCustomer} onClose={() => setEditingCustomer(undefined)} onSave={loadData} />}
      {editingCustomer === null && <CustomerModal customer={null} onClose={() => setEditingCustomer(undefined)} onSave={loadData} />}
      <audio id="remote-audio" autoPlay />
    </div>
  )
}
