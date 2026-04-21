import { useState } from 'react'
import { Headphones, Shield, Globe, Zap, PhoneCall, Users, TrendingUp, BarChart3, CheckCircle, Target, MinusCircle, ArrowRight, Menu } from 'lucide-react'

export default function LandingPage({ onLogin, onRegister }) {
  const [mobileMenu, setMobileMenu] = useState(false)

  return (
    <div className="min-h-screen bg-slate-950 text-white">
      <nav className="fixed top-0 w-full z-50 bg-slate-950/80 backdrop-blur-lg border-b border-slate-800">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-9 h-9 bg-gradient-to-br from-blue-500 to-cyan-400 rounded-xl flex items-center justify-center">
              <Headphones className="w-5 h-5 text-white" />
            </div>
            <span className="text-xl font-bold bg-gradient-to-r from-blue-400 to-cyan-300 bg-clip-text text-transparent">CallFlow</span>
          </div>
          <div className="hidden md:flex items-center gap-8 text-sm text-slate-300">
            <a href="#features" className="hover:text-white">Features</a>
            <a href="#solutions" className="hover:text-white">Solutions</a>
            <a href="#pricing" className="hover:text-white">Pricing</a>
          </div>
          <div className="hidden md:flex items-center gap-3">
            <button onClick={onLogin} className="px-4 py-2 text-sm text-slate-300 hover:text-white">Sign In</button>
            <button onClick={onRegister} className="px-5 py-2 bg-gradient-to-r from-blue-600 to-cyan-500 text-white text-sm font-semibold rounded-lg hover:from-blue-500 hover:to-cyan-400 shadow-lg shadow-blue-500/25">Get Started Free</button>
          </div>
          <button className="md:hidden text-slate-300" onClick={() => setMobileMenu(!mobileMenu)}><Menu className="w-6 h-6" /></button>
        </div>
        {mobileMenu && (
          <div className="md:hidden bg-slate-900 border-b border-slate-800 px-4 py-4 space-y-3">
            <a href="#features" className="block text-slate-300">Features</a>
            <a href="#solutions" className="block text-slate-300">Solutions</a>
            <a href="#pricing" className="block text-slate-300">Pricing</a>
            <div className="flex gap-3 pt-2">
              <button onClick={onLogin} className="px-4 py-2 text-sm text-slate-300 border border-slate-700 rounded-lg">Sign In</button>
              <button onClick={onRegister} className="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg">Get Started</button>
            </div>
          </div>
        )}
      </nav>

      <section className="pt-32 pb-20 px-4">
        <div className="max-w-7xl mx-auto text-center">
          <div className="inline-flex items-center gap-2 bg-blue-500/10 border border-blue-500/20 rounded-full px-4 py-1.5 text-sm text-blue-400 mb-8">
            <Zap className="w-4 h-4" /> Free calls included with every plan
          </div>
          <h1 className="text-5xl sm:text-6xl lg:text-7xl font-extrabold leading-tight mb-6">
            <span className="bg-gradient-to-r from-white via-blue-100 to-cyan-200 bg-clip-text text-transparent">Call Center</span><br />
            <span className="bg-gradient-to-r from-blue-400 to-cyan-400 bg-clip-text text-transparent">& CRM Solution</span>
          </h1>
          <p className="max-w-2xl mx-auto text-lg text-slate-400 mb-10">Professional cloud-based call center with integrated CRM. Make and receive calls from your browser. Track customers, manage tasks, and get real-time exchange recommendations.</p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
            <button onClick={onRegister} className="px-8 py-4 bg-gradient-to-r from-blue-600 to-cyan-500 text-white font-bold rounded-xl hover:from-blue-500 hover:to-cyan-400 shadow-xl shadow-blue-500/30 text-lg flex items-center gap-2">Start Free Trial <ArrowRight className="w-5 h-5" /></button>
            <button onClick={onLogin} className="px-8 py-4 border border-slate-700 text-slate-300 font-semibold rounded-xl hover:bg-slate-800 text-lg">Sign In</button>
          </div>
          <div className="mt-12 flex items-center justify-center gap-8 text-slate-500 text-sm">
            <span className="flex items-center gap-2"><Shield className="w-4 h-4 text-green-500" /> Encrypted</span>
            <span className="flex items-center gap-2"><Globe className="w-4 h-4 text-blue-500" /> Global</span>
            <span className="flex items-center gap-2"><Zap className="w-4 h-4 text-amber-500" /> No setup fee</span>
          </div>
        </div>
      </section>

      <section id="features" className="py-20 px-4 bg-slate-900/50">
        <div className="max-w-7xl mx-auto">
          <h2 className="text-3xl font-bold text-center mb-4">Everything You Need</h2>
          <p className="text-slate-400 text-center mb-12">One platform for calls, CRM, and market insights</p>
          <div className="grid md:grid-cols-3 gap-6">
            {[
              { icon: Headphones, title: 'Call Center solution', desc: 'Browser-based. No hardware needed. or integrate with your local provider', color: 'blue' },
              { icon: Users, title: 'Built-in CRM', desc: 'Customer management, call history, task tracking in one place.', color: 'cyan' },
              { icon: TrendingUp, title: 'Exchange Recommendations', desc: 'Real-time target price and stop-loss for currency markets.', color: 'amber' },
              { icon: PhoneCall, title: 'Free Calls Included', desc: 'Every plan includes free outbound minutes. No hidden fees.', color: 'green' },
              { icon: BarChart3, title: 'Analytics Dashboard', desc: 'Track call metrics and agent performance in real-time.', color: 'purple' },
              { icon: Shield, title: 'Secure & Compliant', desc: 'Encrypted calls, GDPR-ready, full audit trail.', color: 'red' },
            ].map((f, i) => (
              <div key={i} className="bg-slate-800/50 border border-slate-700/50 rounded-2xl p-6 hover:border-slate-600 transition group">
                <div className={`w-12 h-12 bg-${f.color}-500/10 rounded-xl flex items-center justify-center mb-4 group-hover:scale-110 transition`}>
                  <f.icon className={`w-6 h-6 text-${f.color}-400`} />
                </div>
                <h3 className="text-lg font-semibold mb-2">{f.title}</h3>
                <p className="text-slate-400 text-sm">{f.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="solutions" className="py-20 px-4">
        <div className="max-w-7xl mx-auto">
          <h2 className="text-3xl font-bold text-center mb-12">Solutions For Every Team</h2>
          <div className="grid md:grid-cols-2 gap-8">
            {[
              { title: 'Sales Teams', desc: 'Automated dialing, call recording, lead tracking.', icon: TrendingUp },
              { title: 'Customer Support', desc: 'Inbound queue management, call routing, ticket integration.', icon: Headphones },
              { title: 'Financial Advisors', desc: 'Real-time market recommendations with your call workflow.', icon: Target },
              { title: 'Remote Teams', desc: 'Browser-based calling from anywhere. No VPN needed.', icon: Globe },
            ].map((s, i) => (
              <div key={i} className="flex gap-4 bg-slate-900/50 border border-slate-700/50 rounded-2xl p-6 hover:border-blue-500/30 transition">
                <div className="w-12 h-12 bg-blue-500/10 rounded-xl flex items-center justify-center flex-shrink-0"><s.icon className="w-6 h-6 text-blue-400" /></div>
                <div><h3 className="text-lg font-semibold mb-1">{s.title}</h3><p className="text-slate-400 text-sm">{s.desc}</p></div>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="pricing" className="py-20 px-4 bg-slate-900/50">
        <div className="max-w-7xl mx-auto">
          <h2 className="text-3xl font-bold text-center mb-12">Simple Pricing</h2>
          <div className="grid md:grid-cols-3 gap-6 max-w-4xl mx-auto">
            {[
              { name: 'Starter', price: 'Free', period: '', features: ['5 agents','500 free min/mo','Basic CRM','1 recommendation feed'], cta: 'Start Free' },
              { name: 'Business', price: '$29', period: '/agent/mo', features: ['Unlimited agents','5,000 free min/mo','Full CRM + Tasks','All feeds','Priority support'], cta: 'Start Trial', featured: true },
              { name: 'Enterprise', price: 'Custom', period: '', features: ['Unlimited everything','Custom integrations','Dedicated support','SLA guarantee'], cta: 'Contact Sales' },
            ].map((p, i) => (
              <div key={i} className={`rounded-2xl p-6 ${p.featured ? 'bg-gradient-to-b from-blue-600/20 to-cyan-600/10 border-2 border-blue-500/50' : 'bg-slate-800/50 border border-slate-700/50'}`}>
                {p.featured && <div className="text-xs font-bold text-blue-400 mb-2">MOST POPULAR</div>}
                <h3 className="text-xl font-bold">{p.name}</h3>
                <div className="mt-3 mb-6"><span className="text-4xl font-extrabold">{p.price}</span><span className="text-slate-400">{p.period}</span></div>
                <ul className="space-y-2 mb-8 text-sm text-slate-300">{p.features.map((f, j) => <li key={j} className="flex items-center gap-2"><CheckCircle className="w-4 h-4 text-green-500 flex-shrink-0" />{f}</li>)}</ul>
                <button onClick={onRegister} className={`w-full py-3 rounded-xl font-semibold text-sm transition ${p.featured ? 'bg-gradient-to-r from-blue-600 to-cyan-500 text-white shadow-lg shadow-blue-500/25' : 'border border-slate-600 text-slate-300 hover:bg-slate-800'}`}>{p.cta}</button>
              </div>
            ))}
          </div>
        </div>
      </section>

      <footer className="border-t border-slate-800 py-12 px-4">
        <div className="max-w-7xl mx-auto flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-gradient-to-br from-blue-500 to-cyan-400 rounded-lg flex items-center justify-center"><Headphones className="w-4 h-4 text-white" /></div>
            <span className="font-bold text-slate-400">CallFlow</span>
          </div>
          <p className="text-slate-500 text-sm">&copy; {new Date().getFullYear()} CallFlow. All rights reserved.</p>
        </div>
      </footer>
    </div>
  )
}
