import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { useNavigate, Link } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AlertCircle } from "lucide-react";

const FEATURES = [
  "Importação de várias fontes: EFD ICMS, XMLs e planilhas",
  "Importação de NF-e via ERP Bridge com PIS, COFINS e IPI",
  "Dados escriturados visualizados em dashboards",
  "Rastreabilidade NF-e → lançamento EFD para auditoria fiscal",
  "Conformidade SPED layout 020 (vigência Janeiro/2026)",
];

const Login = () => {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [apiVersion, setApiVersion] = useState<string>("...");
  const navigate = useNavigate();
  const { login } = useAuth();

  // Fetch backend version to confirm which build is running
  useState(() => {
    fetch("/api/health")
      .then(r => r.json())
      .then(d => setApiVersion(d.version ?? "?"))
      .catch(() => setApiVersion("offline"));
  });

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setErrorMsg(null);

    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(typeof data === 'string' ? data : "Credenciais inválidas");
      }

      login(data);
      toast.success("Login realizado com sucesso!");
      navigate("/mercadorias");
    } catch (error: any) {
      const msg = error.message || "Erro desconhecido";
      setErrorMsg(msg);
      toast.error(msg);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex">
      {/* ── Painel esquerdo (visível apenas em lg+) ── */}
      <div
        className="hidden lg:flex lg:w-2/5 flex-col justify-between p-10 relative overflow-hidden"
        style={{ background: "#111827" }}
      >
        {/* Círculo decorativo grande — canto superior direito */}
        <div
          className="absolute -top-24 -right-24 w-96 h-96 rounded-full pointer-events-none"
          style={{ background: "radial-gradient(circle, #3b1f3a 0%, #1a0f1e 100%)" }}
        />
        {/* Círculo decorativo menor — canto inferior esquerdo */}
        <div
          className="absolute -bottom-20 -left-20 w-64 h-64 rounded-full pointer-events-none"
          style={{ background: "radial-gradient(circle, #2d1b2e 0%, #111827 100%)" }}
        />

        {/* ── Conteúdo principal ── */}
        <div className="relative z-10">
          {/* Logo Fortes Bezerra */}
          <div className="mb-12">
            <div
              className="inline-block rounded-2xl px-5 py-3"
              style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)" }}
            >
              <img
                src="/logo.png"
                alt="Fortes Bezerra"
                className="h-14 w-auto"
              />
            </div>
          </div>

          {/* Badge */}
          <span
            className="inline-block px-4 py-1.5 rounded-full text-sm uppercase tracking-widest font-semibold"
            style={{
              background: "rgba(255,255,255,0.08)",
              color: "#e5e7eb",
              border: "1px solid rgba(255,255,255,0.15)",
            }}
          >
            Simulador da Reforma Tributária - SPED
          </span>

          {/* Título */}
          <h1 className="text-white text-5xl font-bold leading-tight mt-5">
            FBTax Cloud
            <br />
            Soluções Inteligentes
          </h1>

          {/* Bullets de features */}
          <ul className="mt-6 space-y-3">
            {FEATURES.map((feature) => (
              <li key={feature} className="flex items-center gap-3 text-sm" style={{ color: "#d1d5db" }}>
                <span
                  className="w-1.5 h-1.5 rounded-full shrink-0"
                  style={{ background: "#ef4444" }}
                />
                {feature}
              </li>
            ))}
          </ul>
        </div>

        {/* ── Rodapé do painel ── */}
        <div className="relative z-10">
          {/* Versão — confirma o build ativo */}
          <p className="text-xs" style={{ color: "#6b7280" }}>
            EFD ICMS/IPI v{apiVersion}
          </p>
        </div>
      </div>

      {/* ── Painel direito — formulário de login (inalterado) ── */}
      <div className="flex-1 flex items-center justify-center bg-gray-100 px-4">
        <div className="w-full max-w-[768px]">
          <Card className="w-full shadow-lg">
            <CardHeader className="flex flex-col items-center gap-2 space-y-0 pt-10 pb-8">
              <CardTitle className="text-4xl font-bold">Acesse sua conta</CardTitle>
              <CardDescription className="text-lg">Entre com suas credenciais para continuar</CardDescription>
            </CardHeader>
            <CardContent className="px-10 pb-10">
              {errorMsg && (
                <Alert variant="destructive" className="mb-4">
                  <AlertCircle className="h-4 w-4" />
                  <AlertTitle>Erro</AlertTitle>
                  <AlertDescription>{errorMsg}</AlertDescription>
                </Alert>
              )}

              <form onSubmit={handleLogin} className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="email" className="text-2xl">E-mail</Label>
                <Input
                  id="email"
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="seu@email.com"
                  className="text-2xl h-16"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password" className="text-2xl">Senha</Label>
                <Input
                  id="password"
                  type="password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="text-2xl h-16"
                />
              </div>
              <div className="flex justify-end">
                <Link to="/forgot-password" className="text-lg text-blue-600 hover:underline">
                  Esqueci minha senha
                </Link>
              </div>
              <Button type="submit" className="w-full text-2xl h-16" disabled={isLoading}>
                {isLoading ? "Entrando..." : "Entrar"}
              </Button>
              <div className="text-center text-lg text-gray-500 mt-2">
                Não tem uma conta?{" "}
                <Link to="/register" className="text-blue-600 hover:underline">
                  Crie grátis
                </Link>
              </div>
              </form>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
};

export default Login;
