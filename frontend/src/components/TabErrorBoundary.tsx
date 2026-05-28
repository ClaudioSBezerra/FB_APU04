import React from 'react';

interface Props {
  /** Nome da aba/seção, para identificar onde quebrou. */
  label?: string;
  children: React.ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * Captura erros de runtime no render de uma aba e os exibe NA TELA, em vez de
 * deixar a árvore React desmontar (tela branca). Mostra mensagem + stack para
 * diagnóstico rápido — o usuário copia o texto sem precisar abrir o DevTools.
 */
export class TabErrorBoundary extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    // Loga no console também (para quem usa F12).
    console.error(`[TabErrorBoundary${this.props.label ? ` · ${this.props.label}` : ''}]`, error, info);
  }

  render() {
    const { error } = this.state;
    if (error) {
      return (
        <div className="rounded-md border border-red-300 bg-red-50 p-4 text-sm">
          <p className="font-semibold text-red-800">
            Erro ao renderizar{this.props.label ? ` a aba "${this.props.label}"` : ''}.
          </p>
          <p className="mt-1 text-red-700">{error.message}</p>
          {error.stack && (
            <pre className="mt-3 max-h-64 overflow-auto rounded bg-white/70 p-2 text-[11px] leading-snug text-red-900">
              {error.stack}
            </pre>
          )}
          <p className="mt-2 text-xs text-red-600">
            Copie esta mensagem e envie para o suporte técnico.
          </p>
        </div>
      );
    }
    return this.props.children;
  }
}
