package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/sijms/go-ora/v2" // registra o driver "oracle" via init()
)

// ── Conexão Oracle síncrona dedicada (D-03) ──────────────────────────────────
//
// Diferente do bridge Python (assíncrono, fora do processo Go), esta é a
// primeira conexão Oracle aberta pelo próprio backend Go em tempo de
// requisição. Reaproveita as credenciais já criptografadas em
// erp_bridge_config (oracle_dsn/oracle_usuario/oracle_senha) via
// DecryptFieldWithFallback — nenhum esquema de cripto novo.

// openFiscalOracleConn abre uma conexão Oracle dedicada para companyID,
// lendo e descriptografando as credenciais de erp_bridge_config.
// NUNCA retorna o erro cru de sql.Open ao chamador — mensagens genéricas
// aqui, detalhe completo apenas em log.Printf server-side (T-11-01).
func openFiscalOracleConn(db *sql.DB, companyID string) (*sql.DB, error) {
	var oracleDsn, oracleUsuario, oracleSenha sql.NullString
	err := db.QueryRow(`
		SELECT oracle_dsn, oracle_usuario, oracle_senha
		FROM erp_bridge_config WHERE company_id = $1
	`, companyID).Scan(&oracleDsn, &oracleUsuario, &oracleSenha)
	if err != nil {
		log.Printf("openFiscalOracleConn: erro ao ler erp_bridge_config para company_id=%s: %v", companyID, err)
		return nil, fmt.Errorf("credenciais Oracle não configuradas para a empresa")
	}
	if !oracleDsn.Valid || strings.TrimSpace(oracleDsn.String) == "" {
		return nil, fmt.Errorf("DSN Oracle não configurado para a empresa")
	}

	dsnPlain := DecryptFieldWithFallback(oracleDsn.String)
	usuarioPlain := DecryptFieldWithFallback(oracleUsuario.String)
	senhaPlain := DecryptFieldWithFallback(oracleSenha.String)

	var connStr string
	if strings.HasPrefix(dsnPlain, "oracle://") {
		connStr = dsnPlain
	} else {
		connStr = fmt.Sprintf("oracle://%s:%s@%s", usuarioPlain, senhaPlain, dsnPlain)
	}

	conn, err := sql.Open("oracle", connStr)
	if err != nil {
		log.Printf("openFiscalOracleConn: falha ao inicializar conexão Oracle para company_id=%s: %v", companyID, err)
		return nil, fmt.Errorf("falha ao inicializar conexão Oracle")
	}
	conn.SetMaxOpenConns(5) // deve casar com o cap do semáforo usado nas fases futuras (Pitfall 4)
	return conn, nil
}

// ── POST /api/fiscal/oracle-ping ─────────────────────────────────────────────
// Smoke test admin de alcançabilidade Oracle. Prova, cedo e barato, que o
// Oracle prod/PRODB é alcançável a partir do ambiente de execução real —
// o modo de falha que inviabilizou o FB_TESTESFC como produto standalone.

func FiscalOraclePingHandler(db *sql.DB) http.HandlerFunc {
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
			log.Printf("FiscalOraclePingHandler: GetEffectiveCompanyID falhou para user_id=%s: %v", userID, err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}

		oracleConn, err := openFiscalOracleConn(db, companyID)
		if err != nil {
			log.Printf("FiscalOraclePingHandler: openFiscalOracleConn falhou para company_id=%s: %v", companyID, err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "Falha ao conectar ao Oracle. Verifique as credenciais ERP configuradas.",
			})
			return
		}
		defer oracleConn.Close()

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		if err := oracleConn.PingContext(ctx); err != nil {
			log.Printf("FiscalOraclePingHandler: PingContext falhou para company_id=%s: %v", companyID, err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "Falha ao conectar ao Oracle. Verifique as credenciais ERP configuradas.",
			})
			return
		}

		var dual int
		if err := oracleConn.QueryRowContext(ctx, "SELECT 1 FROM dual").Scan(&dual); err != nil {
			log.Printf("FiscalOraclePingHandler: SELECT 1 FROM dual falhou para company_id=%s: %v", companyID, err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "Falha ao conectar ao Oracle. Verifique as credenciais ERP configuradas.",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}
}
