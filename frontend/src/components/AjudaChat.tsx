import { useState, useRef, useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import * as XLSX from 'xlsx'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useAuth } from '@/contexts/AuthContext'
import {
  MessageCircle, X, Send, Loader2, Trash2, Sparkles, BookOpen, Database,
  ChevronDown, ChevronRight, FileDown,
} from 'lucide-react'

type ChatMode = 'tutorial' | 'dados'

interface DataResult {
  sql: string
  columns: string[]
  rows: Record<string, unknown>[]
  row_count: number
}

interface Message {
  role: 'user' | 'assistant'
  content: string
  data?: DataResult
}

// Rótulos amigáveis por rota (modo tutorial injeta como contexto da página).
const PAGE_LABELS: Record<string, string> = {
  '/dashboard': 'Dashboard',
  '/icms-fronteira': 'ICMS Fronteira — Resumo',
  '/icms-fronteira/antecipacao': 'ICMS Fronteira — Antecipação',
  '/icms-fronteira/st': 'ICMS Fronteira — Substituição Tributária',
  '/icms-fronteira/difal': 'ICMS Fronteira — DIFAL',
  '/icms-fronteira/incentivo': 'ICMS Fronteira — Incentivo',
  '/icms-fronteira/planilha': 'ICMS Fronteira — Planilha de Itens',
  '/icms-fronteira/fretes': 'ICMS Fronteira — Fretes (CT-e)',
  '/icms-fronteira/motor-fiscal': 'ICMS Fronteira — Motor Fiscal',
  '/icms-fronteira/divergencias': 'ICMS Fronteira — Divergências',
  '/icms-fronteira/comparativo': 'ICMS Fronteira — Comparativo de Planilhas',
  '/icms-fronteira/reconciliacao': 'ICMS Fronteira — Reconciliação',
  '/icms-fronteira/legislacao': 'ICMS Fronteira — Legislação',
  '/icms-fronteira/apuracao': 'ICMS Fronteira — Apuração Mensal',
  '/icms-fronteira/extrato': 'ICMS Fronteira — Extrato SEFAZ',
  '/icms-fronteira/contestacoes': 'ICMS Fronteira — Contestações',
  '/icms-fronteira/administrativo': 'ICMS Fronteira — Administrativo',
}

function renderMarkdown(text: string) {
  return text.split('\n').map((line, i) => {
    if (line.startsWith('## ')) {
      return <h3 key={i} className="text-xs font-semibold mt-2 mb-1">{line.slice(3)}</h3>
    }
    const parts = line.split(/(\*\*[^*]+\*\*)/g).map((part, j) =>
      part.startsWith('**') && part.endsWith('**')
        ? <strong key={j}>{part.slice(2, -2)}</strong>
        : part
    )
    const bullet = line.trimStart().startsWith('- ') || line.trimStart().startsWith('• ')
    const numbered = /^\d+\./.test(line.trimStart())
    return (
      <span key={i} className={`block ${bullet || numbered ? 'pl-3' : ''} ${i > 0 ? 'mt-0.5' : ''}`}>
        {parts}
      </span>
    )
  })
}

function formatCell(v: unknown): string {
  if (v === null || v === undefined) return '—'
  if (typeof v === 'object') return JSON.stringify(v)
  if (typeof v === 'number') return v.toLocaleString('pt-BR')
  return String(v)
}

const WELCOME_TUTORIAL: Message = {
  role: 'assistant',
  content: `Olá! Sou o assistente do **FB Tax**. 👋

No modo **Tutorial** posso explicar:
- Como usar as abas de ICMS Fronteira (Antecipação, ST, DIFAL, Comparativo…)
- O flag **COM/SEM inaplicabilidade** e a aprovação de regras
- A **fórmula e o racional do cálculo de uma nota** — informe o número (ex.: *nota 14817*)

Em **Consulta de dados** eu transformo sua pergunta em consulta ao sistema e trago a tabela (com exportação para Excel).`,
}

// Perguntas frequentes (opção de FAQ) — chips clicáveis no modo Tutorial.
const FAQ_SUGESTOES: string[] = [
  'Qual a diferença entre Antecipação, ST e DIFAL?',
  'Para que serve o flag COM / SEM inaplicabilidade?',
  'Como uso o Comparativo de Planilhas?',
  'O que é a "Causa provável" no comparativo?',
  'O IPI entra na base de cálculo?',
  'Como aprovo regras de inaplicabilidade?',
  'O que são os 3 blocos (Mês Anterior / Atual / Não no SPED)?',
  'O que é o mix de alíquotas (4% + 12%)?',
  'Explique a fórmula e o racional do cálculo de uma nota (informe o número)',
]

const WELCOME_DADOS: Message = {
  role: 'assistant',
  content: `Modo **Consulta de dados**. Pergunte em português, ex.:
- "total de ICMS devido por regime no último período"
- "as 10 maiores notas de antecipação"

Eu gero a consulta, executo e mostro a tabela — com botão **Exportar Excel**.`,
}

function ResultTable({ data }: { data: DataResult }) {
  const [showSQL, setShowSQL] = useState(false)
  if (!data.rows || data.rows.length === 0) {
    return <p className="text-xs italic text-muted-foreground mt-2">Nenhum resultado encontrado.</p>
  }

  function exportarExcel() {
    const ws = XLSX.utils.json_to_sheet(data.rows)
    const wb = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(wb, ws, 'Consulta')
    XLSX.writeFile(wb, 'consulta-ia.xlsx')
  }

  return (
    <div className="mt-2 space-y-2">
      <div className="overflow-x-auto border rounded max-h-64">
        <table className="text-[11px] w-full">
          <thead>
            <tr className="bg-gray-100 border-b sticky top-0">
              {data.columns.map(c => (
                <th key={c} className="text-left px-2 py-1 font-semibold whitespace-nowrap">{c}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.rows.map((row, i) => (
              <tr key={i} className="border-b last:border-0 odd:bg-gray-50">
                {data.columns.map(c => (
                  <td key={c} className="px-2 py-1 whitespace-nowrap">{formatCell(row[c])}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="flex items-center gap-3">
        <button
          onClick={exportarExcel}
          className="flex items-center gap-1 text-[11px] font-medium text-emerald-700 hover:text-emerald-800"
        >
          <FileDown className="h-3.5 w-3.5" /> Exportar Excel
        </button>
        <button
          onClick={() => setShowSQL(s => !s)}
          className="flex items-center gap-1 text-[10px] text-muted-foreground hover:text-foreground"
        >
          {showSQL ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
          Ver SQL gerada
        </button>
      </div>
      {showSQL && (
        <pre className="text-[10px] bg-gray-900 text-green-300 p-2 rounded overflow-x-auto whitespace-pre-wrap">{data.sql}</pre>
      )}
    </div>
  )
}

const STORAGE_KEY = 'fbtax-ajuda-chat-v1'

interface PersistedState { mode: ChatMode; messages: Message[] }

function loadPersisted(): PersistedState | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as PersistedState) : null
  } catch { return null }
}

export function AjudaChat() {
  const { token } = useAuth()
  const location = useLocation()
  const [open, setOpen] = useState(false)
  const persisted = loadPersisted()
  const [mode, setMode] = useState<ChatMode>(persisted?.mode ?? 'tutorial')
  const [messages, setMessages] = useState<Message[]>(
    persisted?.messages?.length
      ? persisted.messages
      : [persisted?.mode === 'dados' ? WELCOME_DADOS : WELCOME_TUTORIAL]
  )
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify({ mode, messages })) } catch { /* ignore */ }
  }, [mode, messages])

  useEffect(() => { if (open) setTimeout(() => inputRef.current?.focus(), 50) }, [open])
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages, loading])

  function trocarModo(novo: ChatMode) {
    setMode(novo)
    setMessages([novo === 'tutorial' ? WELCOME_TUTORIAL : WELCOME_DADOS])
  }

  async function send(textArg?: string) {
    const text = (typeof textArg === 'string' ? textArg : input).trim()
    if (!text || loading) return
    setInput('')
    const userMsg: Message = { role: 'user', content: text }
    const history = [...messages, userMsg]
    setMessages(history)
    setLoading(true)

    try {
      if (mode === 'tutorial') {
        const pageCtx = PAGE_LABELS[location.pathname] ?? location.pathname
        const apiMessages = history
          .filter(m => m !== WELCOME_TUTORIAL && m !== WELCOME_DADOS)
          .slice(-6)
          .map(m => ({ role: m.role, content: m.content }))
        const res = await fetch('/api/ai/ajuda', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify({ messages: apiMessages, context: pageCtx }),
        })
        const data = await res.json()
        if (!res.ok) throw new Error(data.error ?? 'Erro desconhecido')
        setMessages(prev => [...prev, { role: 'assistant', content: data.reply }])
      } else {
        const res = await fetch('/api/ai/query', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify({ pergunta: text }),
        })
        const data = await res.json()
        if (!res.ok) throw new Error(data.error ?? data.erro ?? 'Erro desconhecido')
        const n = data.row_count ?? (data.rows?.length ?? 0)
        const reply = n > 0 ? `Encontrei **${n}** resultado(s):` : 'A consulta não retornou resultados.'
        setMessages(prev => [...prev, { role: 'assistant', content: reply, data }])
      }
    } catch (err) {
      setMessages(prev => [...prev, { role: 'assistant', content: `Desculpe, ocorreu um erro: ${(err as Error).message}` }])
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      {!open && (
        <button
          onClick={() => setOpen(true)}
          className="fixed bottom-5 right-5 z-50 flex items-center gap-2 bg-sky-600 text-white shadow-lg rounded-full pl-4 pr-5 py-3 text-sm font-medium hover:bg-sky-700 transition-all hover:scale-105 active:scale-95"
          title="Assistente FB Tax"
        >
          <Sparkles className="h-5 w-5" />
          Assistente
        </button>
      )}

      {open && (
        <div className="fixed bottom-5 right-5 z-50 flex flex-col w-[480px] h-[620px] max-h-[85vh] bg-white border rounded-2xl shadow-2xl overflow-hidden">
          <div className="flex items-center justify-between px-4 py-3 bg-sky-600 text-white shrink-0">
            <div className="flex items-center gap-2">
              <Sparkles className="h-5 w-5" />
              <div>
                <p className="text-sm font-semibold leading-tight">Assistente FB Tax</p>
                <p className="text-[10px] opacity-75 leading-tight">
                  {mode === 'tutorial' ? 'Modo tutorial' : 'Consulta de dados ao sistema'}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-1">
              <button
                onClick={() => setMessages([mode === 'tutorial' ? WELCOME_TUTORIAL : WELCOME_DADOS])}
                className="p-1.5 rounded hover:bg-white/20 transition-colors"
                title="Limpar conversa"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
              <button
                onClick={() => setOpen(false)}
                className="flex items-center gap-1 px-2 py-1 rounded hover:bg-white/20 transition-colors text-xs font-medium"
                title="Sair"
              >
                <X className="h-3.5 w-3.5" /> Sair
              </button>
            </div>
          </div>

          <div className="flex border-b bg-gray-50 shrink-0">
            <button
              onClick={() => trocarModo('tutorial')}
              className={`flex-1 flex items-center justify-center gap-1.5 py-2 text-xs font-medium transition-colors ${
                mode === 'tutorial' ? 'bg-white text-sky-700 border-b-2 border-sky-600' : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              <BookOpen className="h-3.5 w-3.5" /> Tutorial
            </button>
            <button
              onClick={() => trocarModo('dados')}
              className={`flex-1 flex items-center justify-center gap-1.5 py-2 text-xs font-medium transition-colors ${
                mode === 'dados' ? 'bg-white text-sky-700 border-b-2 border-sky-600' : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              <Database className="h-3.5 w-3.5" /> Consulta de dados
            </button>
          </div>

          <div className="flex-1 overflow-y-auto p-3 space-y-3 bg-gray-50">
            {messages.map((msg, i) => (
              <div key={i} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                {msg.role === 'assistant' && (
                  <div className="w-6 h-6 rounded-full bg-sky-600 flex items-center justify-center shrink-0 mr-2 mt-0.5">
                    <MessageCircle className="h-3.5 w-3.5 text-white" />
                  </div>
                )}
                <div className={`${msg.data ? 'max-w-[95%]' : 'max-w-[85%]'} px-3 py-2 rounded-2xl text-xs leading-relaxed ${
                  msg.role === 'user' ? 'bg-sky-600 text-white rounded-tr-sm' : 'bg-white border rounded-tl-sm text-foreground shadow-sm'
                }`}>
                  {msg.role === 'assistant' ? renderMarkdown(msg.content) : msg.content}
                  {msg.data && <ResultTable data={msg.data} />}
                </div>
              </div>
            ))}
            {/* Opção de FAQ — perguntas frequentes clicáveis (modo Tutorial) */}
            {mode === 'tutorial' && messages.length <= 1 && !loading && (
              <div className="space-y-1.5">
                <p className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide flex items-center gap-1">
                  <BookOpen className="h-3 w-3" /> Dúvidas frequentes
                </p>
                <div className="flex flex-wrap gap-1.5">
                  {FAQ_SUGESTOES.map((q, i) => (
                    <button
                      key={i}
                      onClick={() => send(q)}
                      className="text-left text-[11px] px-2.5 py-1 rounded-full border bg-white hover:bg-sky-50 hover:border-sky-300 text-foreground transition-colors"
                    >
                      {q}
                    </button>
                  ))}
                </div>
              </div>
            )}
            {loading && (
              <div className="flex justify-start">
                <div className="w-6 h-6 rounded-full bg-sky-600 flex items-center justify-center shrink-0 mr-2 mt-0.5">
                  <MessageCircle className="h-3.5 w-3.5 text-white" />
                </div>
                <div className="bg-white border rounded-2xl rounded-tl-sm px-3 py-2 shadow-sm">
                  <Loader2 className="h-4 w-4 animate-spin text-sky-600" />
                </div>
              </div>
            )}
            <div ref={bottomRef} />
          </div>

          <div className="shrink-0 border-t bg-white px-3 py-2.5 flex gap-2 items-center">
            <Input
              ref={inputRef}
              value={input}
              onChange={e => setInput(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() } }}
              placeholder={mode === 'tutorial' ? 'Digite sua dúvida...' : 'Pergunte sobre os dados...'}
              className="text-xs h-8 flex-1"
              disabled={loading}
            />
            <Button size="sm" className="h-8 w-8 p-0 shrink-0" onClick={() => send()} disabled={!input.trim() || loading}>
              <Send className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      )}
    </>
  )
}
