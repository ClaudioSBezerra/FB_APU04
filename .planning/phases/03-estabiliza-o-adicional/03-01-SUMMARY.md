# 03-01 Summary — Remoção de Credenciais Hardcoded (STAB-06)

**Status:** COMPLETE  
**Commit:** 14b2d41

## What was done

Removidas todas as credenciais reais dos arquivos versionados:

- `backend/.env` — substituídas senhas Oracle, SMTP e JWT por placeholders (`your_*_here`)
- `installer/.env` — idem para o script de instalação
- `erp-bridge-aws/config-apu04.yaml` — senhas Oracle substituídas por placeholders
- `coolify-env-template.txt` — template atualizado com todas as variáveis necessárias para configuração no Coolify

Os segredos reais continuam funcionando em produção via variáveis de ambiente injetadas pelo Coolify (não versionadas).

## Requirements satisfied

- STAB-06: Nenhum segredo real em arquivos .env* versionados ✓
