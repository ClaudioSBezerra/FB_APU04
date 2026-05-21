package worker

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"

	"fb_apu04/handlers"
)

// StartXMLWorker inicia um pool de 3 goroutines para processar batches de XML assíncronos.
// Segue o mesmo padrão de StartWorker em worker.go:
//   - FOR UPDATE SKIP LOCKED para evitar race conditions entre workers
//   - defer recover() para panic safety (respawna após 5s)
//
// Os batches são gravados em xml_upload_batches com status='pending' e xml_data BYTEA
// (ZIP comprimido dos XMLs) quando len(xmlFiles) > BatchAsyncThreshold.
func StartXMLWorker(db *sql.DB) {
	const WorkerPoolSize = 3

	fmt.Printf("Starting XML Worker Pool (%d workers)...\n", WorkerPoolSize)

	// CRASH RECOVERY: resetar batches que estavam 'processing' ao reiniciar
	res, err := db.Exec(`
		UPDATE xml_upload_batches
		SET status = 'pending', processed_count = 0
		WHERE status = 'processing'`)
	if err == nil {
		count, _ := res.RowsAffected()
		if count > 0 {
			fmt.Printf("XMLWorker Recovery: Reset %d stuck batches from 'processing' to 'pending'\n", count)
		}
	}

	for i := 0; i < WorkerPoolSize; i++ {
		workerID := i + 1
		go xmlWorkerLoop(db, workerID)
	}
}

// xmlWorkerLoop roda com auto-restart em caso de panic (mesmo padrão de workerLoop).
func xmlWorkerLoop(db *sql.DB, id int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("XMLWorker #%d PANIC: %v — reiniciando em 5s...\n", id, r)
			time.Sleep(5 * time.Second)
			go xmlWorkerLoop(db, id)
		}
	}()
	fmt.Printf("XMLWorker #%d iniciado\n", id)
	for {
		processNextXMLBatch(db, id)
		time.Sleep(5 * time.Second)
	}
}

// processNextXMLBatch seleciona e processa o próximo batch pendente.
func processNextXMLBatch(db *sql.DB, workerID int) {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("XMLWorker #%d: erro ao iniciar tx: %v", workerID, err)
		return
	}
	defer tx.Rollback()

	var batchID, companyID, tipo string
	var xmlData []byte
	var competenciaNullable sql.NullString

	err = tx.QueryRow(`
		SELECT id, company_id, tipo, xml_data, competencia
		FROM xml_upload_batches
		WHERE status = 'pending'
		  AND xml_data IS NOT NULL
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(&batchID, &companyID, &tipo, &xmlData, &competenciaNullable)

	if err == sql.ErrNoRows {
		return // nenhum job pendente
	}
	if err != nil {
		log.Printf("XMLWorker #%d: erro ao escanear batch: %v", workerID, err)
		return
	}

	// Marcar como 'processing' dentro da mesma transação
	_, err = tx.Exec(`UPDATE xml_upload_batches SET status='processing' WHERE id=$1`, batchID)
	if err != nil {
		log.Printf("XMLWorker #%d: erro ao marcar processing: %v", workerID, err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("XMLWorker #%d: erro ao commitar pickup: %v", workerID, err)
		return
	}

	competencia := competenciaNullable.String
	log.Printf("XMLWorker #%d: processando batch %s (tipo=%s competencia=%q)", workerID, batchID, tipo, competencia)

	// Descompactar os XMLs do campo xml_data (ZIP em memória)
	xmlFiles, err := extractXMLsFromZip(xmlData)
	if err != nil {
		log.Printf("XMLWorker #%d: erro ao extrair XMLs do batch %s: %v", workerID, batchID, err)
		db.Exec(`UPDATE xml_upload_batches SET status='failed' WHERE id=$1`, batchID)
		return
	}

	// Processar
	handlers.ProcessXMLBatch(db, batchID, companyID, tipo, competencia, xmlFiles)
}

// extractXMLsFromZip é a versão local usada pelo worker (duplica a do handlers para evitar
// dependência circular entre os pacotes worker ↔ handlers).
// Segue as mesmas mitigações de segurança: anti-ZIP-bomb, path traversal.
func extractXMLsFromZip(data []byte) ([]handlers.NamedXML, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("ZIP inválido: %w", err)
	}

	// Chunk stored in DB = BatchChunkSize (2000) XMLs × MaxSingleXMLBytes (10 MB) worst-case.
	// Typical NFe XMLs are 2–20 KB, so real usage is well under 500 MB.
	const maxUncompressed uint64 = 500 * 1024 * 1024  // 500 MB anti-bomb limit for worker chunks
	const maxSingleXML    int64  = 10 * 1024 * 1024   // 10 MB per XML
	var totalUncompressed uint64
	var xmlFiles []handlers.NamedXML

	for _, f := range r.File {
		if strings.Contains(f.Name, "..") {
			continue
		}
		baseName := filepath.Base(f.Name)
		if !strings.EqualFold(filepath.Ext(baseName), ".xml") {
			continue
		}

		totalUncompressed += f.UncompressedSize64
		if totalUncompressed > maxUncompressed {
			return nil, fmt.Errorf("conteúdo do chunk excede limite de 500MB após descompressão")
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("erro ao abrir %s no ZIP: %w", baseName, err)
		}
		xmlData, err := io.ReadAll(io.LimitReader(rc, maxSingleXML+1))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("erro ao ler %s no ZIP: %w", baseName, err)
		}
		if int64(len(xmlData)) > maxSingleXML {
			return nil, fmt.Errorf("arquivo %s excede limite de 10MB por XML", baseName)
		}

		xmlFiles = append(xmlFiles, handlers.NamedXML{Name: baseName, Data: xmlData})
	}

	return xmlFiles, nil
}
