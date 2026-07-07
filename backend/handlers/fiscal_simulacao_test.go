package handlers

import (
	"math"
	"testing"

	"fb_apu04/services"
)

// Caso real validado com a planilha do Claudio + script do pacote (2026-07-07):
// venda 2599,90 → líquido 2076,512 → IBS 2,076512 + CBS 18,688608 →
// nova base 2620,66512 → novo ICMS 314,4798144. O pacote (chamada única)
// retornou BaseCalculo 2620,67 / ValorImposto 314,48.
func TestRunSimulacaoIbsCbs_CasoPlanilha(t *testing.T) {
	it := fiscalItemInput{
		ID:         "item-1",
		XProd:      "TV TESTE",
		VProd:      2599.90,
		VBcIcmsXML: 2599.90,
		VIcmsXML:   311.988,
		PIcmsXML:   12,
		VPisXML:    37.71,
		VCofinsXML: 173.69,
	}
	r1 := &services.FiscalResult{
		AliquotaImposto:     12,
		BaseCalculo:         2620.67, // pacote novo já embute IBS/CBS na base
		BaseCalculoOriginal: 2599.90,
		ValorImposto:        314.48,
		AliquotaIbsUF:       0.1,
		AliquotaCbs:         0.9,
		ValorIbsUF:          2.08,
		ValorCbs:            18.69,
		BaseCalculoIbsCbs:   2076.28,
	}
	trace := &fiscalDebugTrace{}

	sim := runSimulacaoIbsCbs(it, r1, trace, "item teste")

	if sim.Erro != "" {
		t.Fatalf("simulação não deveria falhar: %s", sim.Erro)
	}
	if sim.PrecoLiquido != 2076.51 {
		t.Errorf("PrecoLiquido = %.2f, want 2076.51", sim.PrecoLiquido)
	}
	if sim.ValorIbsSimulado != 2.08 {
		t.Errorf("ValorIbsSimulado = %.2f, want 2.08", sim.ValorIbsSimulado)
	}
	if sim.ValorCbsSimulado != 18.69 {
		t.Errorf("ValorCbsSimulado = %.2f, want 18.69", sim.ValorCbsSimulado)
	}
	if sim.BaseIcmsSimulada != 2620.67 {
		t.Errorf("BaseIcmsSimulada = %.2f, want 2620.67 (base + acréscimo integral)", sim.BaseIcmsSimulada)
	}
	if sim.IcmsSimulado != 314.48 {
		t.Errorf("IcmsSimulado = %.2f, want 314.48 (nova base × 12%%)", sim.IcmsSimulado)
	}
	// Lado pacote = chamada única (sem 2ª ida ao Oracle)
	if sim.IcmsPacote != 314.48 || sim.BaseIcmsPacote != 2620.67 {
		t.Errorf("lado pacote deveria vir da chamada única: base %.2f / valor %.2f", sim.BaseIcmsPacote, sim.IcmsPacote)
	}
	// Simulado × pacote devem fechar neste caso (mesmo método)
	if math.Abs(sim.IcmsSimulado-sim.IcmsPacote) > 0.01 {
		t.Errorf("simulado (%.2f) e pacote (%.2f) deveriam bater no caso da planilha", sim.IcmsSimulado, sim.IcmsPacote)
	}
	if len(trace.entries) == 0 {
		t.Error("trace da simulação não foi registrado")
	}
}

func TestRunSimulacaoIbsCbs_ComST(t *testing.T) {
	// Item com ST: base ST aditiva, valor ST proporcional à variação da base
	it := fiscalItemInput{
		ID: "item-st", VProd: 369.80, VDesc: 9.19,
		VBcIcmsXML: 360.61, VIcmsXML: 43.28, PIcmsXML: 12,
		VBcStXML: 360.61, VStXML: 21.64,
		VPisXML: 5.24, VCofinsXML: 24.12,
	}
	r1 := &services.FiscalResult{
		AliquotaImposto: 12, BaseCalculo: 369.0, BaseCalculoOriginal: 366.0,
		AliquotaIbsUF: 0.1, AliquotaCbs: 0.9,
	}
	sim := runSimulacaoIbsCbs(it, r1, &fiscalDebugTrace{}, "item st")

	if sim.Erro != "" {
		t.Fatalf("simulação não deveria falhar: %s", sim.Erro)
	}
	// líquido = 360,61 − 43,28 − 21,64 − 5,24 − 24,12 = 266,33; acréscimo = 1% = 2,6633
	if sim.PrecoLiquido != 266.33 {
		t.Errorf("PrecoLiquido = %.2f, want 266.33", sim.PrecoLiquido)
	}
	wantBaseSt := round2(360.61 + 2.6633)
	if sim.BaseStSimulada != wantBaseSt {
		t.Errorf("BaseStSimulada = %.2f, want %.2f (base ST + acréscimo)", sim.BaseStSimulada, wantBaseSt)
	}
	if sim.StSimulado <= sim.StOriginal {
		t.Errorf("StSimulado (%.2f) deveria crescer proporcionalmente sobre o original (%.2f)", sim.StSimulado, sim.StOriginal)
	}
}

func TestRunSimulacaoIbsCbs_SemBase(t *testing.T) {
	// Sem alíquotas de Reforma e sem valores IBS/CBS → erro explícito, sem panic
	it := fiscalItemInput{ID: "item-x", VProd: 100, VBcIcmsXML: 100, VIcmsXML: 12}
	r1 := &services.FiscalResult{AliquotaImposto: 12}
	sim := runSimulacaoIbsCbs(it, r1, &fiscalDebugTrace{}, "item x")
	if sim.Erro == "" {
		t.Error("esperava Erro preenchido quando não há IBS/CBS para simular")
	}
}

func TestRunSimulacaoIbsCbs_FallbackValoresPacote(t *testing.T) {
	// Pacote sem alíquotas (zeradas) mas com valores IBS/CBS retornados →
	// usa os valores como acréscimo
	it := fiscalItemInput{
		ID: "item-f", VProd: 1000, VBcIcmsXML: 1000, VIcmsXML: 120, PIcmsXML: 12,
	}
	r1 := &services.FiscalResult{
		AliquotaImposto: 12, ValorIbsUF: 1.0, ValorCbs: 9.0,
	}
	sim := runSimulacaoIbsCbs(it, r1, &fiscalDebugTrace{}, "item f")
	if sim.Erro != "" {
		t.Fatalf("não deveria falhar com fallback: %s", sim.Erro)
	}
	if sim.AcrescimoIbsCbs != 10.0 {
		t.Errorf("AcrescimoIbsCbs = %.2f, want 10.00 (fallback valores do pacote)", sim.AcrescimoIbsCbs)
	}
	if sim.BaseIcmsSimulada != 1010.0 {
		t.Errorf("BaseIcmsSimulada = %.2f, want 1010.00", sim.BaseIcmsSimulada)
	}
	if sim.IcmsSimulado != 121.20 {
		t.Errorf("IcmsSimulado = %.2f, want 121.20", sim.IcmsSimulado)
	}
}

func TestRound2(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{2620.66512, 2620.67},
		{314.4798144, 314.48},
		{-0.005, -0.0}, // half-even do math.Round em negativo: -0.01? Round(-0.5)=-1 → -0.01
		{0, 0},
	}
	for _, tc := range tests[:2] {
		if got := round2(tc.in); got != tc.want {
			t.Errorf("round2(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSanitizeOracleErrForDebug(t *testing.T) {
	if got := sanitizeOracleErrForDebug(nil); got != "" {
		t.Errorf("nil deveria virar string vazia, veio %q", got)
	}
	long := make([]byte, 400)
	for i := range long {
		long[i] = 'x'
	}
	got := sanitizeOracleErrForDebug(errQuebraLinha(string(long)))
	if len(got) > 310 {
		t.Errorf("mensagem não foi truncada: %d chars", len(got))
	}
}

type errQuebraLinha string

func (e errQuebraLinha) Error() string { return "linha1\nlinha2 " + string(e) }

func TestPfImportJobSnapshot(t *testing.T) {
	job := &pfImportJob{
		CompanyID: "c1", Total: 10, Processed: 4,
		Importados: 3, Ignorados: 1,
		Erros: []pfImportErro{{Arquivo: "a.xml", Erro: "chave inválida"}},
	}
	snap := job.snapshot("job-1")
	if snap.JobID != "job-1" || snap.Total != 10 || snap.Processed != 4 || snap.Importados != 3 {
		t.Errorf("snapshot incorreto: %+v", snap)
	}
	if snap.Done {
		t.Error("job não deveria estar done")
	}
	// snapshot deve copiar os erros (não compartilhar o slice)
	snap.Erros[0].Arquivo = "mutado.xml"
	if job.Erros[0].Arquivo != "a.xml" {
		t.Error("snapshot compartilhou o slice de erros com o job (deveria copiar)")
	}
	job.mu.Lock()
	job.Done = true
	job.mu.Unlock()
	if !job.snapshot("job-1").Done {
		t.Error("Done não refletido no snapshot")
	}
}

func TestFiscalDebugTraceAdd(t *testing.T) {
	tr := &fiscalDebugTrace{}
	tr.add("id1", "produto x", "etapa1", "mensagem 1")
	tr.add("", "", "etapa2", "mensagem 2")
	if len(tr.entries) != 2 {
		t.Fatalf("esperava 2 entradas, veio %d", len(tr.entries))
	}
	if tr.entries[0].Etapa != "etapa1" || tr.entries[1].Mensagem != "mensagem 2" {
		t.Errorf("entradas incorretas: %+v", tr.entries)
	}
}
