package handlers

import "testing"

func TestPfsRecalcularIBSCBS_CasoNFe(t *testing.T) {
	// Item 1 da NF-e de exemplo: vBC do gIBSCBS = 2724.95.
	p := pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}
	d := pfsRecalcularIBSCBS(2724.95, p)

	if d.VIbsUF != 1.36 {
		t.Errorf("VIbsUF = %.2f, want 1.36", d.VIbsUF)
	}
	if d.VIbsMun != 1.36 {
		t.Errorf("VIbsMun = %.2f, want 1.36", d.VIbsMun)
	}
	if d.VIBS != 2.72 {
		t.Errorf("VIBS = %.2f, want 2.72", d.VIBS)
	}
	if d.VCBS != 269.77 {
		t.Errorf("VCBS = %.2f, want 269.77", d.VCBS)
	}
	if d.PIbsUF != 0.05 || d.PIbsMun != 0.05 || d.PCbs != 9.90 {
		t.Errorf("percentuais incorretos: %+v", d)
	}
}

func TestPfsRecalcularIBSCBS_CasoCTe(t *testing.T) {
	// Bloco único do CT-e de exemplo: vBC = 2747.18.
	p := pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}
	d := pfsRecalcularIBSCBS(2747.18, p)

	if d.VIbsUF != 1.37 {
		t.Errorf("VIbsUF = %.2f, want 1.37", d.VIbsUF)
	}
	if d.VIBS != 2.74 {
		t.Errorf("VIBS = %.2f, want 2.74", d.VIBS)
	}
	if d.VCBS != 271.97 {
		t.Errorf("VCBS = %.2f, want 271.97", d.VCBS)
	}
}

func TestPfsRecalcularIBSCBS_BaseZero(t *testing.T) {
	p := pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}
	d := pfsRecalcularIBSCBS(0, p)
	if d.VIbsUF != 0 || d.VIbsMun != 0 || d.VIBS != 0 || d.VCBS != 0 {
		t.Errorf("base zero deveria zerar tudo, veio %+v", d)
	}
}

func TestPfsTotaisIBSCBS_Somar(t *testing.T) {
	var tot pfsTotaisIBSCBS
	p := pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}
	tot.Somar(pfsRecalcularIBSCBS(2724.95, p))
	tot.Somar(pfsRecalcularIBSCBS(7.45, p))
	tot.Somar(pfsItemDepois{TemIBSCBS: false}) // item sem IBSCBS não deve somar nada

	if tot.VCBS != round2(269.77+round2(7.45*9.90/100)) {
		t.Errorf("soma de VCBS incorreta: %.4f", tot.VCBS)
	}
}
