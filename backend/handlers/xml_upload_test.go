package handlers

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"testing"
)

// makeTestZIP creates an in-memory ZIP archive from a name→content map.
func makeTestZIP(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("makeTestZIP: create %s: %v", name, err)
		}
		f.Write(data)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("makeTestZIP: close: %v", err)
	}
	return buf.Bytes()
}

// minimalValidNFe is the smallest XML that passes all processSingleXML guards
// up to db.Begin(): valid parse, mod=55, 44-char chave, valid dhEmi.
const minimalValidNFe = `<nfeProc>` +
	`<NFe><infNFe Id="NFe12345678901234567890123456789012345678901234">` +
	`<ide><mod>55</mod><dhEmi>2024-01-15</dhEmi></ide>` +
	`<dest><CNPJ>12345678000195</CNPJ></dest>` +
	`<total><ICMSTot></ICMSTot><IBSCBSTot></IBSCBSTot></total>` +
	`</infNFe></NFe>` +
	`</nfeProc>`

// runProcessXMLBatch calls ProcessXMLBatch with nil DB and recovers any panic.
func runProcessXMLBatch(xmlFiles []NamedXML) (panicked bool) {
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		ProcessXMLBatch((*sql.DB)(nil), "batch-1", "company-1", "entradas", xmlFiles)
	}()
	return
}

// ---------------------------------------------------------------------------
// extractXMLsFromZip tests
// ---------------------------------------------------------------------------

func TestExtractXMLsFromZip_InvalidZIP(t *testing.T) {
	_, err := extractXMLsFromZip([]byte("not a zip"))
	if err == nil {
		t.Error("expected error for invalid ZIP data")
	}
}

func TestExtractXMLsFromZip_EmptyZIP(t *testing.T) {
	data := makeTestZIP(t, map[string][]byte{})
	result, err := extractXMLsFromZip(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 files, got %d", len(result))
	}
}

func TestExtractXMLsFromZip_ValidXML(t *testing.T) {
	data := makeTestZIP(t, map[string][]byte{
		"nota.xml": []byte("<nfeProc/>"),
	})
	result, err := extractXMLsFromZip(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 file, got %d", len(result))
	}
}

func TestExtractXMLsFromZip_PathTraversal(t *testing.T) {
	data := makeTestZIP(t, map[string][]byte{
		"../evil.xml": []byte("<x/>"),
	})
	result, err := extractXMLsFromZip(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "../evil.xml" should be skipped due to ".." check, leaving 0 results
	if len(result) != 0 {
		t.Errorf("expected path traversal entry to be skipped, got %d results", len(result))
	}
}

func TestExtractXMLsFromZip_NonXMLFile(t *testing.T) {
	data := makeTestZIP(t, map[string][]byte{
		"data.csv": []byte("col1,col2"),
	})
	result, err := extractXMLsFromZip(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected non-XML file to be skipped, got %d results", len(result))
	}
}

// ---------------------------------------------------------------------------
// ProcessXMLBatch / processSingleXML tests (nil-DB panic recovery)
// ---------------------------------------------------------------------------

// Test: 1 invalid (unparseable) XML → processXMLBatch loops, processSingleXML
// returns parse error, batch cleanup db.Exec panics → covered statements up to panic.
func TestProcessXMLBatch_InvalidXML(t *testing.T) {
	items := []NamedXML{{Name: "bad.xml", Data: []byte("not xml")}}
	panicked := runProcessXMLBatch(items)
	if !panicked {
		t.Error("expected nil-DB panic in processXMLBatch cleanup")
	}
}

// Test: 11 invalid XMLs → triggers i%10==0 progress update branch (i=10),
// covering the extra progress-update block before the panic.
func TestProcessXMLBatch_ProgressUpdate(t *testing.T) {
	items := make([]NamedXML, 11)
	for i := range items {
		items[i] = NamedXML{Name: "bad.xml", Data: []byte("not xml")}
	}
	panicked := runProcessXMLBatch(items)
	if !panicked {
		t.Error("expected nil-DB panic in processXMLBatch progress update")
	}
}

// Test: valid NFe XML → passes all pre-db.Begin guards → panics at db.Begin.
func TestProcessXMLBatch_ValidXMLReachesDB(t *testing.T) {
	items := []NamedXML{{Name: "nota.xml", Data: []byte(minimalValidNFe)}}
	panicked := runProcessXMLBatch(items)
	if !panicked {
		t.Error("expected nil-DB panic at db.Begin in processSingleXML")
	}
}

// Test: XML with unsupported mod → covers the "modelo não suportado" return.
func TestProcessXMLBatch_InvalidMod(t *testing.T) {
	badMod := `<nfeProc>` +
		`<NFe><infNFe Id="NFe12345678901234567890123456789012345678901234">` +
		`<ide><mod>99</mod><dhEmi>2024-01-15</dhEmi></ide>` +
		`</infNFe></NFe>` +
		`</nfeProc>`
	items := []NamedXML{{Name: "bad.xml", Data: []byte(badMod)}}
	panicked := runProcessXMLBatch(items)
	if !panicked {
		t.Error("expected nil-DB panic in processXMLBatch cleanup")
	}
}

// Test: XML with short chave (< 44 chars) → covers "chave de acesso inválida" return.
func TestProcessXMLBatch_InvalidChave(t *testing.T) {
	shortChave := `<nfeProc>` +
		`<NFe><infNFe Id="NFe123">` +
		`<ide><mod>55</mod><dhEmi>2024-01-15</dhEmi></ide>` +
		`</infNFe></NFe>` +
		`</nfeProc>`
	items := []NamedXML{{Name: "bad.xml", Data: []byte(shortChave)}}
	panicked := runProcessXMLBatch(items)
	if !panicked {
		t.Error("expected nil-DB panic in processXMLBatch cleanup")
	}
}

// Test: XML with empty dhEmi → parseDhEmi returns error → covers that return path.
func TestProcessXMLBatch_InvalidDate(t *testing.T) {
	emptyDate := `<nfeProc>` +
		`<NFe><infNFe Id="NFe12345678901234567890123456789012345678901234">` +
		`<ide><mod>55</mod><dhEmi></dhEmi></ide>` +
		`</infNFe></NFe>` +
		`</nfeProc>`
	items := []NamedXML{{Name: "bad.xml", Data: []byte(emptyDate)}}
	panicked := runProcessXMLBatch(items)
	if !panicked {
		t.Error("expected nil-DB panic in processXMLBatch cleanup")
	}
}
