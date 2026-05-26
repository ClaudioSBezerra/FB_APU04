import { BrowserRouter, Routes, Route, Link, Navigate, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'
import ImportarEFD from './pages/ImportarEFD'
import Mercadorias from './pages/Mercadorias'
import MercadoriasXML from './pages/MercadoriasXML'
import OperacoesSimplesNacional from './pages/OperacoesSimplesNacional'
import Dashboard from './pages/Dashboard'
import ExecutiveSummary from './pages/ExecutiveSummary'
import ConsultaInteligente from './pages/ConsultaInteligente'
import TabelaAliquotas from './pages/TabelaAliquotas'
import TabelaCFOP from './pages/TabelaCFOP'
import TabelaFornSimples from './pages/TabelaFornSimples'
import ApelidosFiliais from './pages/ApelidosFiliais'
import GestaoAmbiente from './pages/GestaoAmbiente'
import Managers from './pages/Managers'
import ConsultaNFesEntradas from './pages/ConsultaNFesEntradas'
import ConsultaCTesEntradas from './pages/ConsultaCTesEntradas'
import ImportarXMLsEntrada from './pages/ImportarXMLsEntrada'
import ImportarXMLsSaida from './pages/ImportarXMLsSaida'
import ImportarXMLsCTe from './pages/ImportarXMLsCTe'
import PainelXMLs from './pages/PainelXMLs'
import ConciliacaoBridgeXML from './pages/ConciliacaoBridgeXML'
import ComparativoEFDvsXML from './pages/ComparativoEFDvsXML'
import RelatorioSaneamento from './pages/RelatorioSaneamento'
import ERPBridgeConfig from './pages/ERPBridgeConfig'
import ERPBridgeLogs from './pages/ERPBridgeLogs'
import ERPBridgeCredenciais from './pages/ERPBridgeCredenciais'
import AdminUsers from './pages/AdminUsers'
import LimparDados from './pages/LimparDados'
import ReformaParametros from './pages/ReformaParametros'
import EmpresaParametros from './pages/EmpresaParametros'
import Reforma11CreditosBloqueados from './pages/Reforma11CreditosBloqueados'
import Reforma12Reprecificacao from './pages/Reforma12Reprecificacao'
import Reforma13RankingFornecedores from './pages/Reforma13RankingFornecedores'
import Reforma14SplitPayment from './pages/Reforma14SplitPayment'
import Reforma22CfopAnalysis from './pages/Reforma22CfopAnalysis'
import Reforma21NcmAnalysis  from './pages/Reforma21NcmAnalysis'
import Reforma23UfDestino    from './pages/Reforma23UfDestino'
import Reforma24B2bB2c       from './pages/Reforma24B2bB2c'
import IcmsFronteira from './pages/IcmsFronteira'
import Login from './pages/Login'
import Register from './pages/Register'
import ForgotPassword from './pages/ForgotPassword'
import ResetPassword from './pages/ResetPassword'
import { AppRail } from '@/components/AppRail'
import { FilialSelector } from '@/components/FilialSelector'
import { CompanySwitcher } from '@/components/CompanySwitcher'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { FilialProvider } from './contexts/FilialContext'
import { getActiveModule, modules } from '@/lib/navigation'
import { cn } from '@/lib/utils'

const queryClient = new QueryClient()

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

  return (
    <header className="flex items-center justify-between h-12 border-b bg-white px-4 shrink-0">
      <span className="text-sm font-semibold text-foreground">
        {moduleCfg?.label ?? 'FBTax Cloud'}
      </span>
      <div className="flex items-center gap-2">
        <FilialSelector />
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
            <Routes>
              <Route path="/" element={<Navigate to="/mercadorias" replace />} />

              {/* Simulador da Reforma Tributária - SPED */}
              <Route path="/importar-efd"                      element={<ImportarEFD />} />
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
              <Route path="/icms-fronteira/planilha"           element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/fretes"             element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/motor-fiscal"       element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/divergencias"       element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/reconciliacao"      element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/legislacao"         element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/regras"             element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/extrato"            element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/contestacoes"       element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/apuracao"           element={<IcmsFronteira />} />
              <Route path="/icms-fronteira/administrativo"     element={<IcmsFronteira />} />

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
          </div>
        </main>
      </div>
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
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App
