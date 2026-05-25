package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Parâmetros de Empresa — logo e template de relatório de antecipação
// ---------------------------------------------------------------------------

const maxLogoSize = 5 << 20     // 5 MB
const maxTemplateSize = 20 << 20 // 20 MB

type EmpresaParametrosInfo struct {
	TemLogo            bool   `json:"tem_logo"`
	LogoMime           string `json:"logo_mime,omitempty"`
	LogoNome           string `json:"logo_nome,omitempty"`
	TemTemplateAntecip bool   `json:"tem_template_antecip"`
	TemplateAntecipNome string `json:"template_antecip_nome,omitempty"`
}

// GetEmpresaParametrosHandler — GET /api/config/empresa/parametros
// Retorna metadados (sem os bytes) sobre logo e template.
func GetEmpresaParametrosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		var info EmpresaParametrosInfo
		var logoData []byte
		var templateData []byte
		err = db.QueryRow(`
			SELECT logo_data, COALESCE(logo_mime,''), COALESCE(logo_nome,''),
			       template_antecip_data, COALESCE(template_antecip_nome,'')
			FROM companies WHERE id = $1`, companyID).
			Scan(&logoData, &info.LogoMime, &info.LogoNome,
				&templateData, &info.TemplateAntecipNome)
		if err != nil && err != sql.ErrNoRows {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		info.TemLogo = len(logoData) > 0
		info.TemTemplateAntecip = len(templateData) > 0

		json.NewEncoder(w).Encode(info)
	}
}

// ServeEmpresaLogoHandler — GET /api/config/empresa/logo
// Serve os bytes da logo como imagem.
func ServeEmpresaLogoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var logoData []byte
		var mime string
		err = db.QueryRow(`SELECT logo_data, COALESCE(logo_mime,'image/jpeg') FROM companies WHERE id = $1`, companyID).
			Scan(&logoData, &mime)
		if err != nil || len(logoData) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(logoData) //nolint:errcheck
	}
}

// UploadEmpresaLogoHandler — POST /api/config/empresa/logo (admin)
func UploadEmpresaLogoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxLogoSize)
		if err := r.ParseMultipartForm(maxLogoSize); err != nil {
			jsonErr(w, http.StatusBadRequest, "Arquivo muito grande (máx 5 MB)")
			return
		}
		file, header, err := r.FormFile("logo")
		if err != nil {
			jsonErr(w, http.StatusBadRequest, "Campo 'logo' obrigatório")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler arquivo")
			return
		}

		mime := header.Header.Get("Content-Type")
		if mime == "" {
			mime = "image/jpeg"
		}

		_, err = db.Exec(`UPDATE companies SET logo_data=$1, logo_mime=$2, logo_nome=$3 WHERE id=$4`,
			data, mime, header.Filename, companyID)
		if err != nil {
			log.Printf("UploadEmpresaLogo error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao salvar logo")
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true, "nome": header.Filename, "mime": mime, "bytes": len(data),
		})
	}
}

// ServeTemplateAntecipHandler — GET /api/config/empresa/template-antecipacao
func ServeTemplateAntecipHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var data []byte
		var nome string
		err = db.QueryRow(`SELECT template_antecip_data, COALESCE(template_antecip_nome,'template.pdf') FROM companies WHERE id=$1`, companyID).
			Scan(&data, &nome)
		if err != nil || len(data) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`"`)
		w.Write(data) //nolint:errcheck
	}
}

// UploadTemplateAntecipHandler — POST /api/config/empresa/template-antecipacao (admin)
func UploadTemplateAntecipHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxTemplateSize)
		if err := r.ParseMultipartForm(maxTemplateSize); err != nil {
			jsonErr(w, http.StatusBadRequest, "Arquivo muito grande (máx 20 MB)")
			return
		}
		file, header, err := r.FormFile("template")
		if err != nil {
			jsonErr(w, http.StatusBadRequest, "Campo 'template' obrigatório")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler arquivo")
			return
		}

		_, err = db.Exec(`UPDATE companies SET template_antecip_data=$1, template_antecip_nome=$2 WHERE id=$3`,
			data, header.Filename, companyID)
		if err != nil {
			log.Printf("UploadTemplateAntecip error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao salvar template")
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true, "nome": header.Filename, "bytes": len(data),
		})
	}
}
