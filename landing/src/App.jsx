const BOT_URL = 'https://t.me/ChatVault1Bot'

const TAGS = [
  { label: 'idea', color: 'bg-violet-500/15 text-violet-300 border-violet-500/30' },
  { label: 'decision', color: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30' },
  { label: 'action-item', color: 'bg-amber-500/15 text-amber-300 border-amber-500/30' },
  { label: 'question', color: 'bg-sky-500/15 text-sky-300 border-sky-500/30' },
  { label: 'document', color: 'bg-rose-500/15 text-rose-300 border-rose-500/30' },
]

const CHAT_MOCK = [
  { from: 'Mira', text: "Let's ship the onboarding redesign next sprint.", tag: TAGS[1] },
  { from: 'Dao', text: 'What if we A/B test the new flow first?', tag: TAGS[3] },
  { from: 'Mira', text: 'Use a feature flag so we can roll back fast.', tag: TAGS[0] },
  { from: 'Dao', text: '🎤 voice message — transcribed automatically', tag: TAGS[2] },
]

const FEATURES = [
  {
    title: 'Auto-tagging with AI',
    desc: 'Every message is silently classified as an idea, decision, action item, question, document, or noise — no manual labeling.',
    icon: (
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 0 0-2.456 2.456Z"
      />
    ),
  },
  {
    title: 'Voice messages, transcribed',
    desc: 'Drop a voice note in the chat and ChatVault transcribes it and tags it like any other message.',
    icon: (
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 18.75a6 6 0 0 0 6-6v-1.5m-6 7.5a6 6 0 0 1-6-6v-1.5m6 7.5v3.75m-3.75 0h7.5M12 15.75a3 3 0 0 1-3-3V4.5a3 3 0 1 1 6 0v8.25a3 3 0 0 1-3 3Z"
      />
    ),
  },
  {
    title: 'Daily summaries',
    desc: 'A clean digest of the day’s conversation lands in your chat automatically, every day.',
    icon: (
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5m-9-6h.008v.008H12v-.008Z"
      />
    ),
  },
  {
    title: 'Export to Notion',
    desc: 'Send your daily summary straight to a Notion database with one command — decisions, action items, and ideas, organized.',
    icon: (
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M9 12h6m-6 4h6m-7 5h8a2 2 0 0 0 2-2V7.414a2 2 0 0 0-.586-1.414l-3.414-3.414A2 2 0 0 0 12.586 2H8a2 2 0 0 0-2 2v15a2 2 0 0 0 2 2Z"
      />
    ),
  },
]

const COMMANDS = [
  { cmd: '/summary', desc: "Async summary of today's conversation" },
  { cmd: '/ideas', desc: 'Idea messages from the last 7 days' },
  { cmd: '/decisions', desc: 'Decision messages from the last 7 days' },
  { cmd: '/actions', desc: 'Action items from the last 7 days' },
  { cmd: '/notion', desc: 'Connect a Notion database to this chat' },
  { cmd: '/export', desc: "Export today's summary to Notion" },
]

const STATS = [
  { value: '6', label: 'AI tag categories' },
  { value: '24/7', label: 'Background tagging' },
  { value: '<1 min', label: 'To add to a group' },
  { value: '1', label: 'Command to export to Notion' },
]

const STEPS = [
  { n: '01', t: 'Add the bot to your group', d: 'Invite @ChatVault1Bot to any Telegram group chat.' },
  { n: '02', t: 'Chat as usual', d: 'ChatVault stores messages and tags them in the background with AI — text or voice.' },
  { n: '03', t: 'Get summaries & insights', d: 'Pull up ideas, decisions, and action items on demand, or get a daily digest automatically.' },
]

function Icon({ children, className = 'h-6 w-6' }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.5} className={className}>
      {children}
    </svg>
  )
}

function TagPill({ label, color }) {
  return (
    <span className={`inline-flex items-center rounded-full border px-3 py-1 text-xs font-medium whitespace-nowrap ${color}`}>
      {label}
    </span>
  )
}

function Glow() {
  return (
    <div className="pointer-events-none fixed inset-0 -z-10 overflow-hidden">
      <div className="absolute -top-40 left-1/2 h-[36rem] w-[36rem] -translate-x-1/2 rounded-full bg-violet-600/25 blur-[120px]" />
      <div className="absolute top-[10rem] right-[-10%] h-[28rem] w-[28rem] rounded-full bg-fuchsia-600/15 blur-[120px]" />
      <div className="absolute top-[45rem] left-[-5%] h-[26rem] w-[26rem] rounded-full bg-sky-600/15 blur-[120px]" />
      <div className="absolute top-[80rem] right-[5%] h-[26rem] w-[26rem] rounded-full bg-emerald-600/10 blur-[120px]" />
      <div className="absolute top-[115rem] left-[15%] h-[24rem] w-[24rem] rounded-full bg-amber-600/10 blur-[120px]" />
    </div>
  )
}

function Nav() {
  return (
    <header className="sticky top-0 z-50 border-b border-white/5 bg-[#0b0c10]/70 backdrop-blur-xl">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
        <span className="flex items-center gap-2 text-lg font-semibold tracking-tight text-white">
          <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-violet-400 to-violet-600 text-sm font-bold text-white shadow-md shadow-violet-500/30">
            C
          </span>
          ChatVault
        </span>
        <nav className="hidden gap-8 text-sm text-gray-400 sm:flex">
          <a href="#features" className="transition hover:text-white">Features</a>
          <a href="#how-it-works" className="transition hover:text-white">How it works</a>
          <a href="#commands" className="transition hover:text-white">Commands</a>
        </nav>
        <a
          href={BOT_URL}
          target="_blank"
          rel="noreferrer"
          className="rounded-full bg-violet-500 px-4 py-2 text-sm font-semibold text-white shadow-md shadow-violet-500/30 transition hover:bg-violet-400"
        >
          Add to Telegram
        </a>
      </div>
    </header>
  )
}

function ChatMock() {
  return (
    <div className="w-full max-w-sm rounded-2xl border border-white/10 bg-[#11121a]/90 p-1 shadow-2xl shadow-black/40 backdrop-blur">
      <div className="flex items-center gap-2 border-b border-white/5 px-4 py-3">
        <span className="h-2.5 w-2.5 rounded-full bg-red-400/70" />
        <span className="h-2.5 w-2.5 rounded-full bg-yellow-400/70" />
        <span className="h-2.5 w-2.5 rounded-full bg-green-400/70" />
        <span className="ml-2 text-xs font-medium text-gray-400">#product-team</span>
      </div>
      <div className="flex flex-col gap-3 p-4 text-left">
        {CHAT_MOCK.map((m, i) => (
          <div key={i} className="flex flex-col gap-1">
            <div className="flex items-baseline gap-2">
              <span className="text-xs font-semibold text-white">{m.from}</span>
              <span className="text-[11px] text-gray-500">{9 + i}:0{i}</span>
            </div>
            <p className="text-sm text-gray-300">{m.text}</p>
            <TagPill {...m.tag} />
          </div>
        ))}
      </div>
    </div>
  )
}

function Hero() {
  return (
    <section className="relative mx-auto max-w-5xl px-6 pt-20 pb-16 sm:pt-28">
      <div className="grid items-center gap-12 lg:grid-cols-[1.1fr_0.9fr]">
        <div className="text-center lg:text-left">
          <p className="mb-4 inline-flex items-center gap-2 rounded-full border border-violet-500/30 bg-violet-500/10 px-3 py-1 text-xs font-medium uppercase tracking-widest text-violet-300">
            <span className="h-1.5 w-1.5 rounded-full bg-violet-400" />
            Telegram bot
          </p>
          <h1 className="text-4xl font-semibold leading-tight tracking-tight text-white sm:text-5xl lg:text-6xl">
            Your group chat already knows{' '}
            <span className="text-violet-400">everything.</span>{' '}
            ChatVault remembers it.
          </h1>
          <p className="mx-auto mt-6 max-w-xl text-lg text-gray-400 lg:mx-0">
            Drop ChatVault into any Telegram group and it quietly turns the conversation into a
            searchable, AI-tagged knowledge base — ideas, decisions, and action items included.
          </p>
          <div className="mt-10 flex flex-col items-center justify-center gap-3 sm:flex-row lg:justify-start">
            <a
              href={BOT_URL}
              target="_blank"
              rel="noreferrer"
              className="rounded-full bg-violet-500 px-6 py-3 text-base font-semibold text-white shadow-lg shadow-violet-500/30 transition hover:scale-[1.03] hover:bg-violet-400"
            >
              Start with @ChatVault1Bot
            </a>
            <a
              href="#how-it-works"
              className="rounded-full border border-white/10 px-6 py-3 text-base font-semibold text-gray-300 transition hover:border-white/30 hover:text-white"
            >
              See how it works
            </a>
          </div>
        </div>
        <div className="flex justify-center lg:justify-end">
          <ChatMock />
        </div>
      </div>
    </section>
  )
}

function Stats() {
  return (
    <section className="border-y border-white/5 bg-white/[0.02]">
      <div className="mx-auto grid max-w-5xl grid-cols-2 gap-8 px-6 py-12 sm:grid-cols-4">
        {STATS.map((s) => (
          <div key={s.label} className="text-center">
            <p className="text-3xl font-semibold text-white">{s.value}</p>
            <p className="mt-1 text-sm text-gray-400">{s.label}</p>
          </div>
        ))}
      </div>
    </section>
  )
}

function Features() {
  return (
    <section
      id="features"
      className="relative mx-auto max-w-5xl px-6 py-20 before:absolute before:inset-x-0 before:top-1/2 before:-z-10 before:h-[40rem] before:-translate-y-1/2 before:bg-gradient-to-b before:from-transparent before:via-fuchsia-500/[0.04] before:to-transparent"
    >
      <div className="text-center">
        <p className="text-sm font-semibold uppercase tracking-widest text-violet-400">Features</p>
        <h2 className="mt-3 text-3xl font-semibold tracking-tight text-white">
          Everything your chat says, organized automatically
        </h2>
      </div>
      <div className="mt-12 grid gap-5 sm:grid-cols-2">
        {FEATURES.map((f) => (
          <div
            key={f.title}
            className="group rounded-2xl border border-white/10 bg-white/[0.03] p-6 transition hover:-translate-y-1 hover:border-violet-500/30 hover:bg-white/[0.05]"
          >
            <div className="inline-flex h-11 w-11 items-center justify-center rounded-xl bg-violet-500/10 text-violet-300 transition group-hover:bg-violet-500/20">
              <Icon className="h-5 w-5">{f.icon}</Icon>
            </div>
            <h3 className="mt-4 text-lg font-semibold text-white">{f.title}</h3>
            <p className="mt-2 text-sm leading-relaxed text-gray-400">{f.desc}</p>
          </div>
        ))}
      </div>
    </section>
  )
}

function HowItWorks() {
  return (
    <section id="how-it-works" className="mx-auto max-w-5xl px-6 py-20">
      <div className="text-center">
        <p className="text-sm font-semibold uppercase tracking-widest text-violet-400">Process</p>
        <h2 className="mt-3 text-3xl font-semibold tracking-tight text-white">How it works</h2>
      </div>
      <div className="relative mt-14 grid gap-10 sm:grid-cols-3">
        <div className="absolute top-5 left-0 hidden h-px w-full bg-gradient-to-r from-violet-500/0 via-violet-500/40 to-violet-500/0 sm:block" />
        {STEPS.map((s) => (
          <div key={s.n} className="relative text-center sm:text-left">
            <span className="relative z-10 inline-flex h-10 w-10 items-center justify-center rounded-full border border-violet-500/40 bg-[#0b0c10] text-sm font-semibold text-violet-300">
              {s.n}
            </span>
            <h3 className="mt-4 text-lg font-semibold text-white">{s.t}</h3>
            <p className="mt-2 text-sm leading-relaxed text-gray-400">{s.d}</p>
          </div>
        ))}
      </div>
    </section>
  )
}

function Commands() {
  return (
    <section id="commands" className="border-y border-white/5 bg-white/[0.02]">
      <div className="mx-auto max-w-5xl px-6 py-20">
        <div className="text-center">
          <p className="text-sm font-semibold uppercase tracking-widest text-violet-400">Reference</p>
          <h2 className="mt-3 text-3xl font-semibold tracking-tight text-white">Commands</h2>
        </div>
        <div className="mx-auto mt-12 max-w-2xl overflow-hidden rounded-2xl border border-white/10 bg-[#0e0f15] shadow-xl shadow-black/30">
          <div className="flex items-center gap-2 border-b border-white/5 bg-white/[0.02] px-4 py-3">
            <span className="h-2.5 w-2.5 rounded-full bg-red-400/70" />
            <span className="h-2.5 w-2.5 rounded-full bg-yellow-400/70" />
            <span className="h-2.5 w-2.5 rounded-full bg-green-400/70" />
            <span className="ml-2 text-xs font-medium text-gray-500">@ChatVault1Bot</span>
          </div>
          <div className="divide-y divide-white/5">
            {COMMANDS.map((c) => (
              <div key={c.cmd} className="flex items-center justify-between gap-4 px-6 py-4 transition hover:bg-white/[0.03]">
                <code className="rounded bg-violet-500/10 px-2 py-1 text-sm text-violet-300">{c.cmd}</code>
                <span className="text-right text-sm text-gray-400">{c.desc}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

function CTA() {
  return (
    <section className="mx-auto max-w-5xl px-6 py-20">
      <div className="relative overflow-hidden rounded-3xl border border-white/10 bg-gradient-to-br from-violet-500/15 via-white/[0.02] to-sky-500/10 px-8 py-16 text-center">
        <div className="pointer-events-none absolute -top-24 left-1/2 h-64 w-64 -translate-x-1/2 rounded-full bg-violet-500/20 blur-3xl" />
        <h2 className="relative text-3xl font-semibold tracking-tight text-white">
          Stop losing good ideas in the scroll.
        </h2>
        <p className="relative mx-auto mt-4 max-w-md text-gray-400">
          Add ChatVault to your group in under a minute. No setup required to get started.
        </p>
        <a
          href={BOT_URL}
          target="_blank"
          rel="noreferrer"
          className="relative mt-8 inline-flex rounded-full bg-violet-500 px-6 py-3 text-base font-semibold text-white shadow-lg shadow-violet-500/30 transition hover:scale-[1.03] hover:bg-violet-400"
        >
          Open @ChatVault1Bot
        </a>
      </div>
    </section>
  )
}

function Footer() {
  return (
    <footer className="border-t border-white/5 px-6 py-8 text-center text-sm text-gray-500">
      ChatVault — Telegram group knowledge-base bot.
    </footer>
  )
}

function App() {
  return (
    <div className="relative min-h-screen overflow-x-hidden">
      <Glow />
      <Nav />
      <main>
        <Hero />
        <Stats />
        <Features />
        <HowItWorks />
        <Commands />
        <CTA />
      </main>
      <Footer />
    </div>
  )
}

export default App
