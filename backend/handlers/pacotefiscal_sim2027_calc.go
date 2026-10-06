// pacotefiscal_sim2027_calc.go — motor de recálculo da "Simulação 2027":
// zera PIS/COFINS/IPI e recalcula IBS/CBS sobre a base (vBC) já existente no
// bloco gIBSCBS do XML, usando os parâmetros globais do módulo.
package handlers

// pfsParametros — alíquotas globais de teste usadas na simulação 2027.
type pfsParametros struct {
	AliqIbsUfPct  float64
	AliqIbsMunPct float64
	AliqCbsPct    float64
}

// pfsItemAntes — valores de PIS/COFINS/IPI/IBS/CBS do item como estavam no
// XML original (para o preview antes/depois).
type pfsItemAntes struct {
	VPIS      float64
	VCOFINS   float64
	VIPI      float64
	VBCIbsCbs float64
	TemIBSCBS bool
	PIbsUF    float64
	VIbsUF    float64
	PIbsMun   float64
	VIbsMun   float64
	VIBS      float64
	PCbs      float64
	VCBS      float64
}

// pfsItemDepois — valores recalculados.
type pfsItemDepois struct {
	VPIS      float64 // sempre 0
	VCOFINS   float64 // sempre 0
	VIPI      float64 // sempre 0
	TemIBSCBS bool
	PIbsUF    float64
	VIbsUF    float64
	PIbsMun   float64
	VIbsMun   float64
	VIBS      float64
	PCbs      float64
	VCBS      float64
}

// pfsRecalcularIBSCBS aplica as novas alíquotas sobre a base vBC já existente
// no XML. vIBS é a soma de UF+Mun (mesma semântica observada nos exemplos
// reais: gIBSUF.vIBSUF + gIBSMun.vIBSMun == gIBS.vIBS).
func pfsRecalcularIBSCBS(vBC float64, p pfsParametros) pfsItemDepois {
	vIbsUF := round2(vBC * p.AliqIbsUfPct / 100)
	vIbsMun := round2(vBC * p.AliqIbsMunPct / 100)
	vCbs := round2(vBC * p.AliqCbsPct / 100)
	return pfsItemDepois{
		TemIBSCBS: true,
		PIbsUF:    p.AliqIbsUfPct,
		VIbsUF:    vIbsUF,
		PIbsMun:   p.AliqIbsMunPct,
		VIbsMun:   vIbsMun,
		VIBS:      round2(vIbsUF + vIbsMun),
		PCbs:      p.AliqCbsPct,
		VCBS:      vCbs,
	}
}

// pfsItemPreview — um item (ou, no caso do CT-e, o documento inteiro) com
// antes/depois, para a resposta JSON do endpoint "analisar".
type pfsItemPreview struct {
	NItem  int               `json:"n_item,omitempty"`
	CProd  string            `json:"c_prod,omitempty"`
	XProd  string            `json:"x_prod,omitempty"`
	Antes  pfsItemAntesJSON  `json:"antes"`
	Depois pfsItemDepoisJSON `json:"depois"`
}

// Os campos de IBS/CBS NUNCA usam "omitempty": um valor recalculado sobre
// base pequena arredonda legitimamente para 0.00 (ex.: vBC=7.45 × 0.05% =
// 0.00), e "omitempty" apagaria a chave inteira do JSON nesse caso — achado
// em teste manual (item 2 do XML de exemplo). "tem_ibs_cbs" já é o sinalizador
// correto de presença/ausência do bloco.
type pfsItemAntesJSON struct {
	VPIS      float64 `json:"v_pis"`
	VCOFINS   float64 `json:"v_cofins"`
	VIPI      float64 `json:"v_ipi"`
	TemIBSCBS bool    `json:"tem_ibs_cbs"`
	VBC       float64 `json:"v_bc_ibs_cbs"`
	PIbsUF    float64 `json:"p_ibs_uf"`
	VIbsUF    float64 `json:"v_ibs_uf"`
	PIbsMun   float64 `json:"p_ibs_mun"`
	VIbsMun   float64 `json:"v_ibs_mun"`
	VIBS      float64 `json:"v_ibs"`
	PCbs      float64 `json:"p_cbs"`
	VCBS      float64 `json:"v_cbs"`
}

type pfsItemDepoisJSON struct {
	VPIS      float64 `json:"v_pis"`
	VCOFINS   float64 `json:"v_cofins"`
	VIPI      float64 `json:"v_ipi"`
	TemIBSCBS bool    `json:"tem_ibs_cbs"`
	PIbsUF    float64 `json:"p_ibs_uf"`
	VIbsUF    float64 `json:"v_ibs_uf"`
	PIbsMun   float64 `json:"p_ibs_mun"`
	VIbsMun   float64 `json:"v_ibs_mun"`
	VIBS      float64 `json:"v_ibs"`
	PCbs      float64 `json:"p_cbs"`
	VCBS      float64 `json:"v_cbs"`
}

// pfsTotaisIBSCBS — soma dos valores por item (NF-e) usada para recompor
// <total><IBSCBSTot> ao gerar o XML.
type pfsTotaisIBSCBS struct {
	VIbsUF  float64
	VIbsMun float64
	VIBS    float64
	VCBS    float64
}

func (t *pfsTotaisIBSCBS) Somar(d pfsItemDepois) {
	if !d.TemIBSCBS {
		return
	}
	t.VIbsUF += d.VIbsUF
	t.VIbsMun += d.VIbsMun
	t.VIBS += d.VIBS
	t.VCBS += d.VCBS
}
