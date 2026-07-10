package handlers

import (
	"database/sql"
	"errors"
	"testing"
)

func TestIsTransientOraclePackageErr(t *testing.T) {
	if isTransientOraclePackageErr(nil) {
		t.Error("nil deveria ser false")
	}
	for _, code := range []string{"ORA-04068", "ORA-04061", "ORA-04065", "ORA-06508"} {
		if !isTransientOraclePackageErr(errors.New("algo " + code + " no meio")) {
			t.Errorf("%s deveria ser transitório", code)
		}
	}
	if isTransientOraclePackageErr(errors.New("ORA-00001 unique violation")) {
		t.Error("ORA-00001 não é transitório")
	}
}

func TestNullIntToString(t *testing.T) {
	if got := nullIntToString(sql.NullInt64{Valid: false}); got != "" {
		t.Errorf("NULL → %q", got)
	}
	if got := nullIntToString(sql.NullInt64{Int64: 1, Valid: true}); got != "1" {
		t.Errorf("1 → %q", got)
	}
}
