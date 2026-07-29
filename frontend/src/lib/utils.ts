import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}


export function formatCurrency(value: number): string {
  return new Intl.NumberFormat('pt-BR', {
    style: 'currency',
    currency: 'BRL',
  }).format(value);
}

// Espelha a validação do backend (xml_upload.go): MM/YYYY, mês 1-12, ano >= 2000.
export function isValidCompetencia(value: string): boolean {
  const match = /^(\d{2})\/(\d{4})$/.exec(value.trim());
  if (!match) return false;
  const month = Number(match[1]);
  const year = Number(match[2]);
  return month >= 1 && month <= 12 && year >= 2000;
}
