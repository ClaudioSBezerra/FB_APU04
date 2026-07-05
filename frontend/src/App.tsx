import { lazy, Suspense } from 'react'
import { BrowserRouter, Routes, Route, Link, Navigate, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'
// Páginas carregadas sob demanda (code-splitting): cada rota vira um chunk
// próprio, tirando ~2 MB do bundle de entrada. Libs pesadas usadas só por
// algumas páginas (recharts, react-simple-maps, xlsx) saem do entry junto.
const ImportarEFD = lazy(() => import('./pages/ImportarEFD'))
const AuditoriaEFD = lazy(() => import('./pages/AuditoriaEFD'))
const Mercadorias = lazy(() => import('./pages/Mercadorias'))
const MercadoriasXML = lazy(() => import('./pages/MercadoriasXML'))
const OperacoesSimplesNacional = lazy(() => import('./pages/OperacoesSimplesNacional'))
const Dashboard = lazy(() => import('./pages/Dashboard'))
const ExecutiveSummary = lazy(() => import('./pages/ExecutiveSummary'))
const ConsultaInteligente = lazy(() => import('./pages/ConsultaInteligente'))
const TabelaAliquotas = lazy(() => import('./pages/TabelaAliquotas'))
const TabelaCFOP = lazy(() => import('./pages/TabelaCFOP'))
const TabelaFornSimples = lazy(() => import('./pages/TabelaFornSimples'))
const ApelidosFiliais = lazy(() => import('./pages/ApelidosFiliais'))
const GestaoAmbiente = lazy(() => import('./pages/GestaoAmbiente'))
const Managers = lazy(() => import('./pages/Managers'))
const ConsultaNFesEntradas = lazy(() => import('./pages/ConsultaNFesEntradas'))
const ConsultaCTesEntradas = lazy(() => import('./pages/ConsultaCTesEntradas'))
const ImportarXMLsEntrada = lazy(() => import('./pages/ImportarXMLsEntrada'))
const ImportarXMLsSaida = lazy(() => import('./pages/ImportarXMLsSaida'))
const ImportarXMLsCTe = lazy(() => import('./pages/ImportarXMLsCTe'))
const PainelXMLs = lazy(() => import('./pages/PainelXMLs'))
const ConciliacaoBridgeXML = lazy(() => import('./pages/ConciliacaoBridgeXML'))
const ComparativoEFDvsXML = lazy(() => import('./pages/ComparativoEFDvsXML'))
const RelatorioSaneamento = lazy(() => import('./pages/RelatorioSaneamento'))
const ERPBridgeConfig = lazy(() => import('./pages/ERPBridgeConfig'))
const ERPBridgeLogs = lazy(() => import('./pages/ERPBridgeLogs'))
const ERPBridgeCredenciais = lazy(() => import('./pages/ERPBridgeCredenciais'))
const ImportarViaERP = lazy(() => import('./pages/ImportarViaERP'))
const ImportacaoERPLogs = lazy(() => import('./pages/ImportacaoERPLogs'))
const AdminUsers = lazy(() => import('./pages/AdminUsers'))
const LimparDados = lazy(() => import('./pages/LimparDados'))
const ReformaParametros = lazy(() => import('./pages/ReformaParametros'))
const EmpresaParametros = lazy(() => import('./pages/EmpresaParametros'))
const Reforma11CreditosBloqueados = lazy(() => import('./pages/Reforma11CreditosBloqueados'))
const Reforma12Reprecificacao = lazy(() => import('./pages/Reforma12Reprecificacao'))
const Reforma13RankingFornecedores = lazy(() => import('./pages/Reforma13RankingFornecedores'))
const Reforma14SplitPayment = lazy(() => import('./pages/Reforma14SplitPayment'))
const Reforma22CfopAnalysis = lazy(() => import('./pages/Reforma22CfopAnalysis'))
const Reforma21NcmAnalysis = lazy(() => import('./pages/Reforma21NcmAnalysis'))
const Reforma23UfDestino = lazy(() => import('./pages/Reforma23UfDestino'))
const Reforma24B2bB2c = lazy(() => import('./pages/Reforma24B2bB2c'))
const IcmsFronteira = lazy(() => import('./pages/IcmsFronteira'))
const ComparacaoFiscal = lazy(() => import('./pages/ComparacaoFiscal'))
const ImportarXMLPacoteFiscal = lazy(() => import('./pages/ImportarXMLPacoteFiscal'))
const Login = lazy(() => import('./pages/Login'))
const Register = lazy(() => import('./pages/Register'))
const ForgotPassword = lazy(() => import('./pages/ForgotPassword'))
const ResetPassword = lazy(() => import('./pages/ResetPassword'))
import { AppRail } from '@/components/AppRail'
import { AjudaChat } from '@/components/AjudaChat'
import { FilialSelector } from '@/components/FilialSelector'
import { CompanySwitcher } from '@/components/CompanySwitcher'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { FilialProvider } from './contexts/FilialContext'
import { getActiveModule, modules } from '@/lib/navigation'
import { cn } from '@/lib/utils'

const queryClient = new QueryClient()

// Fallback exibido enquanto o chunk da rota (lazy) é baixado.
function PageLoader() {
  return (
    <div className="flex items-center justify-center h-[60vh]">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-muted border-t-primary" />
    </div>
  )
}

function ComingSoon({ title }: { title: string }) {
  return (
    <div className="flex flex-col items-center justify-center h-[50vh] space-y-4">
      <h1 className="text-2xl font-bold text-muted-foreground">{title}</h1>
      <p className="text-sm text-muted-foreground">Este módulo está em desenvolvimento.</p>
    </div>
  )
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, loading } = useAuth()
  const location = useLocation()
  if (loading) return null
  if (!isAuthenticated) return <Navigate to="/login" state={{ from: location }} replace />
  return <>{children}</>
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, loading, user } = useAuth()
  const location = useLocation()
  if (loading) return null
  if (!isAuthenticated) return <Navigate to="/login" state={{ from: location }} replace />
  if (user?.role !== 'admin') return <Navigate to="/" replace />
  return <>{children}</>
}

// Rota inicial de cada módulo — usada para redirecionar o usuário para o
// primeiro módulo liberado pelas suas personas quando ele tenta acessar
// (ou cai por padrão em) um módulo que não tem.
const MODULE_HOME: Record<string, string> = {
  simulador:    '/mercadorias',
  notas:        '/importacoes/xml/entradas',
  painel:       '/painel/xmls',
  reforma:      '/reforma/creditos',
  fronteira:    '/icms-fronteira',
  auditoria:    '/auditoria-efd',
  pacotefiscal: '/pacote-fiscal/comparacao',
}

// Gate por módulo (personas): envolve todas as rotas do AppLayout. Config fica
// aberto a todos — as abas sensíveis lá dentro continuam adminOnly.
function ModuleGate({ children }: { children: React.ReactNode }) {
  const { user, hasModule } = useAuth()
  const location = useLocation()
  const moduleId = getActiveModule(location.pathname)

  if (moduleId === 'config' || hasModule(moduleId)) return <>{children}</>

  const firstAllowed = user?.modules?.find(m => MODULE_HOME[m])
  return <Navigate to={firstAllowed ? MODULE_HOME[firstAllowed] : '/config/aliquotas'} replace />
}

// ── Barra de abas por módulo ─────────────────────────────────────────────────
function ModuleTabs() {
  const location  = useLocation()
  const { user }  = useAuth()
  const isAdmin   = user?.role === 'admin'
  const moduleId  = getActiveModule(location.pathname)
  const moduleCfg = modules[moduleId]

  if (!moduleCfg || moduleCfg.tabs.length === 0) return null

  const visibleTabs = moduleCfg.tabs.filter(t => !t.adminOnly || isAdmin)

  return (
    <div key={moduleId} className="border-b bg-white px-4 flex items-center gap-0.5 overflow-x-auto shrink-0 h-10">
      {visibleTabs.map(tab => {
        const isActive   = location.pathname === tab.path
        const isDisabled = tab.disabled
        return isDisabled ? (
          <span
            key={tab.path}
            className="px-3 py-1.5 text-xs rounded-md text-muted-foreground/50 cursor-not-allowed whitespace-nowrap"
          >
            {tab.label}
          </span>
        ) : (
          <Link
            key={tab.path}
            to={tab.path}
            className={cn(
              'px-3 py-1.5 text-xs rounded-md whitespace-nowrap transition-colors',
              isActive
                ? tab.danger
                  ? 'bg-red-50 text-red-700 font-semibold'
                  : 'bg-primary/10 text-primary font-semibold'
                : tab.danger
                  ? 'text-red-500 hover:bg-red-50 hover:text-red-700'
                  : 'text-muted-foreground hover:bg-gray-100 hover:text-foreground'
            )}
          >
            {tab.label}
          </Link>
        )
      })}
    </div>
  )
}

// ── Cabeçalho (módulo + controles globais) ───────────────────────────────────
function AppHeader() {
  const location  = useLocation()
  const moduleId  = getActiveModule(location.pathname)
  const moduleCfg = modules[moduleId]
  const { company } = useAuth()

  return (
    <header className="flex items-center justify-between h-12 border-b bg-white px-4 shrink-0">
      <span className="text-sm font-semibold text-foreground">
        {moduleCfg?.label ?? 'FBTax Cloud'}
      </span>
      <div className="flex items-center gap-2">
        {/* Seletor de filiais só faz sentido no Simulador da Reforma Tributária. */}
        {moduleId === 'simulador' && <FilialSelector />}

        {/* Empresa ativa — sempre visível, independente de quantas empresas existem */}
        {company && (
          <span
            className="flex items-center gap-1.5 text-xs font-medium text-sky-700 bg-sky-50 border border-sky-200 px-2.5 py-1 rounded-full max-w-[220px]"
            title={company}
          >
            <svg className="h-3 w-3 shrink-0 text-sky-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M3 21V7l9-4 9 4v14M9 21V12h6v9" />
            </svg>
            <span className="truncate">{company}</span>
          </span>
        )}

        <CompanySwitcher compact />
      </div>
    </header>
  )
}

// ── Layout principal ─────────────────────────────────────────────────────────
function AppLayout() {
  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <AppRail />
      <div className="flex flex-col flex-1 min-w-0">
        <AppHeader />
        <ModuleTabs />
        <main className="flex-1 overflow-auto">
          <div className="p-4">
            <Suspense fallback={<PageLoader />}>
            <ModuleGate>
            <Routes>
              <Route path="/" element={<Navigate to="/mercadorias" replace />} />

              {/* Simulador da Reforma Tributária - SPED */}
              <Route path="/importar-efd"                      element={<ImportarEFD />} />
              <Route path="/auditoria-efd"                     element={<AuditoriaEFD />} />
              <Route path="/pacote-fiscal/importar"            element={<AdminRoute><ImportarXMLPacoteFiscal /></AdminRoute>} />
              <Route path="/pacote-fiscal/comparacao"          element={<AdminRoute><ComparacaoFiscal /></AdminRoute>} />
              <Route path="/mercadorias/xml"                   element={<MercadoriasXML />} />
              <Route path="/mercadorias"                       element={<Mercadorias />} />
              <Route path="/operacoes/simples"                 element={<OperacoesSimplesNacional />} />
              <Route path="/dashboards"                        element={<Dashboard />} />
              <Route path="/relatorios/resumo-executivo"       element={<ExecutiveSummary />} />
              <Route path="/relatorios/consulta-inteligente"   element={<ConsultaInteligente />} />

              {/* Notas Importadas */}
              <Route path="/apuracao/entrada/notas"            element={<ConsultaNFesEntradas />} />
              <Route path="/apuracao/cte-entrada/notas"        element={<ConsultaCTesEntradas />} />
              <Route path="/apuracao/nfse"                     element={<ComingSoon title="NFS-e Entradas" />} />
              <Route path="/importacoes/erp-bridge"            element={<AdminRoute><ERPBridgeConfig /></AdminRoute>} />
              <Route path="/importacoes/erp-bridge/logs"       element={<AdminRoute><ERPBridgeLogs /></AdminRoute>} />
              <Route path="/importacoes/erp-bridge-xml"        element={<AdminRoute><ImportarViaERP /></AdminRoute>} />
              <Route path="/importacoes/erp-bridge-xml/logs"   element={<AdminRoute><ImportacaoERPLogs /></AdminRoute>} />
              <Route path="/importacoes/xml/entradas"          element={<ProtectedRoute><ImportarXMLsEntrada /></ProtectedRoute>} />
              <Route path="/importacoes/xml/saidas"            element={<ProtectedRoute><ImportarXMLsSaida /></ProtectedRoute>} />
              <Route path="/importacoes/xml/ctes"              element={<ProtectedRoute><ImportarXMLsCTe /></ProtectedRoute>} />
              <Route path="/painel/xmls"                       element={<PainelXMLs />} />
              <Route path="/relatorios/saneamento-cclasstrib"  element={<RelatorioSaneamento />} />
              <Route path="/conciliacao/bridge-xml"            element={<ConciliacaoBridgeXML />} />
              <Route path="/comparativo/efd-xml"               element={<ComparativoEFDvsXML />} />

              {/* Análise Reforma Tributária */}
              <Route path="/reforma/parametros"         element={<ReformaParametros />} />
              <Route path="/config/reforma-parametros"  element={<ReformaParametros />} />
              <Route path="/config/empresa"             element={<EmpresaParametros />} />
              <Route path="/reforma/creditos"           element={<Reforma11CreditosBloqueados />} />
              <Route path="/reforma/reprecificacao"     element={<Reforma12Reprecificacao />} />
              <Route path="/reforma/ranking"            element={<Reforma13RankingFornecedores />} />
              <Route path="/reforma/split-payment"      element={<Reforma14SplitPayment />} />
              <Route path="/reforma/cfop"               element={<Reforma22CfopAnalysis />} />
              <Route path="/reforma/ncm"                element={<Reforma21NcmAnalysis />} />
              <Route path="/reforma/uf-destino"         element={<Reforma23UfDestino />} />
              <Route path="/reforma/b2b-b2c"            element={<Reforma24B2bB2c />} />

              {/* ICMS Fronteira */}
              <Route path="/icms-fronteira"                    element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/antecipacao"        element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/st"                 element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/difal"              element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/incentivo"          element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/planilha"           element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/fretes"             element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/motor-fiscal"       element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/divergencias"       element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/comparativo"        element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/reconciliacao"      element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/legislacao"         element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/regras"             element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/extrato"            element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/contestacoes"       element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/apuracao"           element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/administrativo"     element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/validacao"          element={<IcmsFronteira />} />

              {/* Configurações */}
              <Route path="/config/aliquotas"                  element={<TabelaAliquotas />} />
              <Route path="/config/cfop"                       element={<TabelaCFOP />} />
              <Route path="/config/forn-simples"               element={<TabelaFornSimples />} />
              <Route path="/config/apelidos-filiais"           element={<ApelidosFiliais />} />
              <Route path="/config/gestores"                   element={<Managers />} />
              <Route path="/config/ambiente"                   element={<ProtectedRoute><GestaoAmbiente /></ProtectedRoute>} />
              <Route path="/config/erp-bridge"                 element={<AdminRoute><ERPBridgeCredenciais /></AdminRoute>} />
              <Route path="/config/usuarios"                   element={<AdminRoute><AdminUsers /></AdminRoute>} />
              <Route path="/config/limpar-dados"               element={<AdminRoute><LimparDados /></AdminRoute>} />
            </Routes>
            </ModuleGate>
            </Suspense>
          </div>
        </main>
      </div>
      <AjudaChat />
      <Toaster />
    </div>
  )
}

// ── App root ─────────────────────────────────────────────────────────────────
function App() {
  console.log('App Version: 1.0.0 — FB_APU04 Simulador da Reforma Tributária - SPED')
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <AuthProvider>
          <Suspense fallback={<PageLoader />}>
          <Routes>
            <Route path="/login"           element={<Login />} />
            <Route path="/register"        element={<Register />} />
            <Route path="/forgot-password" element={<ForgotPassword />} />
            <Route path="/reset-senha"     element={<ResetPassword />} />
            <Route path="/*" element={
              <ProtectedRoute>
                <FilialProvider>
                  <AppLayout />
                </FilialProvider>
              </ProtectedRoute>
            } />
          </Routes>
          </Suspense>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App
