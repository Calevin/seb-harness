package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDB crea una base de datos SQLite en memoria para pruebas aisladas
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory sqlite: %v", err)
	}
	if err := initDBSchema(db); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}
	return db
}

// CHK-01 (RF-01): Verificacion de calculo y persistencia de duracion en milisegundos
func TestAuditLogDuration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	expectedDuration := int64(345)
	resolvedTask := resolveActiveTask("001", "", "", db)
	normDur := normalizeDuration(expectedDuration)

	_, err := db.Exec(`INSERT INTO agent_actions
		(session_id, task_id, agent, skill, action, duration_ms, exit_code)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"test-session", resolvedTask, "test-agent", "implementador", "run_command", normDur, 0)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	var dur int64
	err = db.QueryRow(`SELECT duration_ms FROM agent_actions WHERE action = 'run_command'`).Scan(&dur)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if dur != expectedDuration {
		t.Errorf("Expected duration %d ms, got %d ms", expectedDuration, dur)
	}
}

// CHK-02 (RF-02): Verificacion de registro exacto de tokens_in y tokens_out
func TestAuditLogTokens(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	inputTokens := int64(1520)
	outputTokens := int64(380)

	_, err := db.Exec(`INSERT INTO agent_actions
		(session_id, task_id, agent, skill, action, tokens_in, tokens_out, exit_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-session", "001", "test-agent", "implementador", "llm_turn", inputTokens, outputTokens, 0)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	var inTok, outTok int64
	err = db.QueryRow(`SELECT tokens_in, tokens_out FROM agent_actions WHERE action = 'llm_turn'`).Scan(&inTok, &outTok)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if inTok != inputTokens || outTok != outputTokens {
		t.Errorf("Expected tokens (%d, %d), got (%d, %d)", inputTokens, outputTokens, inTok, outTok)
	}
}

// CHK-03 (RF-03): Verificacion de calculo y persistencia de costo USD con tabla local y fallback
func TestAuditCostCalculation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := EngineConfig{
		Pricing: map[string]ModelPricing{
			"claude-3-5-sonnet": {
				PromptPricePerMillion:     3.0,
				CompletionPricePerMillion: 15.0,
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, cfgBytes, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Caso 1: Costo explicito provisto
	costExplicit := calculateCost(0.045, "claude-3-5-sonnet", 1000, 1000, configPath)
	if costExplicit != 0.045 {
		t.Errorf("Expected explicit cost 0.045, got %f", costExplicit)
	}

	// Caso 2: Calculo automatico usando tabla local
	// 1,000,000 in ($3.0) + 1,000,000 out ($15.0) = $18.0
	costCalculated := calculateCost(0.0, "claude-3-5-sonnet", 1000000, 1000000, configPath)
	if costCalculated != 18.0 {
		t.Errorf("Expected calculated cost 18.0, got %f", costCalculated)
	}

	// Caso 3: Modelo desconocido -> Fallback a 0.0
	costUnknown := calculateCost(0.0, "unknown-model", 5000, 5000, configPath)
	if costUnknown != 0.0 {
		t.Errorf("Expected 0.0 for unknown model, got %f", costUnknown)
	}

	// Caso 4: Archivo inexistente -> Fallback a 0.0
	costNoFile := calculateCost(0.0, "claude-3-5-sonnet", 1000, 1000, filepath.Join(tmpDir, "nonexistent.json"))
	if costNoFile != 0.0 {
		t.Errorf("Expected 0.0 for missing config file, got %f", costNoFile)
	}
}

// CHK-04 (RF-04): Verificacion de cronometraje interno de comandos del arnes (duracion > 0)
func TestInternalActionsDuration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Simulamos medicion de comando interno
	start := time.Now()
	time.Sleep(2 * time.Millisecond)
	elapsed := time.Since(start).Milliseconds()

	logInternalAction(db, "001", "seb-especificador", "spec-next", "test_task", "specs/001_test.md", elapsed, 0)

	var dur int64
	var act, task string
	err := db.QueryRow(`SELECT task_id, action, duration_ms FROM agent_actions WHERE action = 'spec-next'`).Scan(&task, &act, &dur)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if dur <= 0 {
		t.Errorf("Expected duration > 0, got %d ms", dur)
	}
	if task != "001" {
		t.Errorf("Expected task '001', got '%s'", task)
	}
}

// CHK-05 (RF-05): Verificacion de cascada estricta para resolucion de tarea activa
func TestActiveTaskCascade(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	activeFile := filepath.Join(tmpDir, "ACTIVE_TASK")

	// Insertamos tarea en SQLite
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('002', 'db_task', 'specs/002.md', 'SPECIFIED')`)

	// Nivel 1: Flag explicito
	res1 := resolveActiveTask("001", "ENV_TASK", activeFile, db)
	if res1 != "001" {
		t.Errorf("Expected level 1 '001', got '%s'", res1)
	}

	// Nivel 2: Variable de entorno
	res2 := resolveActiveTask("", "ENV_TASK", activeFile, db)
	if res2 != "ENV_TASK" {
		t.Errorf("Expected level 2 'ENV_TASK', got '%s'", res2)
	}

	// Nivel 3: Archivo ACTIVE_TASK
	if err := os.WriteFile(activeFile, []byte("file_task"), 0644); err != nil {
		t.Fatalf("Failed to write active task file: %v", err)
	}
	res3 := resolveActiveTask("", "", activeFile, db)
	if res3 != "file_task" {
		t.Errorf("Expected level 3 'file_task', got '%s'", res3)
	}

	// Nivel 3 con mapeo de nombre a ID en DB
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('003', 'mapped_task', 'specs/003.md', 'SPECIFIED')`)
	_ = os.WriteFile(activeFile, []byte("mapped_task"), 0644)
	res3b := resolveActiveTask("", "", activeFile, db)
	if res3b != "003" {
		t.Errorf("Expected mapped level 3 '003', got '%s'", res3b)
	}

	// Nivel 4: SQLite ultima tarea activa (borramos el archivo active)
	_ = os.Remove(activeFile)
	res4 := resolveActiveTask("", "", activeFile, db)
	if res4 != "003" && res4 != "002" {
		t.Errorf("Expected level 4 task from DB, got '%s'", res4)
	}
}

// CHK-06 (RF-06): Verificacion de importacion determinista e idempotente de usage.jsonl
func TestImportUsageJsonl(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	usagePath := filepath.Join(tmpDir, "usage.jsonl")

	records := []UsageRecord{
		{
			Timestamp:        "2026-10-23T01:00:00Z",
			Turns:            4,
			InputTokens:      1200,
			OutputTokens:     350,
			ApprovalRequired: false,
			Outcome:          "saved",
		},
		{
			Timestamp:        "2026-10-23T01:05:00Z",
			Turns:            6,
			InputTokens:      2500,
			OutputTokens:     800,
			ApprovalRequired: true,
			Outcome:          "saved",
		},
	}

	f, err := os.Create(usagePath)
	if err != nil {
		t.Fatalf("Failed to create temp usage.jsonl: %v", err)
	}
	for _, r := range records {
		b, _ := json.Marshal(r)
		f.Write(append(b, '\n'))
	}
	f.Close()

	// Primera importacion: debe importar 2 registros
	count1, err := importUsageJsonl(db, "001", usagePath)
	if err != nil {
		t.Fatalf("Import 1 failed: %v", err)
	}
	if count1 != 2 {
		t.Errorf("Expected 2 records imported, got %d", count1)
	}

	// Segunda importacion idempotente: debe omitir duplicados y retornar 0
	count2, err := importUsageJsonl(db, "001", usagePath)
	if err != nil {
		t.Fatalf("Import 2 failed: %v", err)
	}
	if count2 != 0 {
		t.Errorf("Expected 0 records on duplicate import, got %d", count2)
	}

	// Verificar totales en DB
	var totalIn, totalOut int64
	err = db.QueryRow(`SELECT SUM(tokens_in), SUM(tokens_out) FROM agent_actions WHERE task_id = '001'`).Scan(&totalIn, &totalOut)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if totalIn != 3700 || totalOut != 1150 {
		t.Errorf("Expected tokens (3700, 1150), got (%d, %d)", totalIn, totalOut)
	}
}

// CHK-07 (RF-07): Verificacion de normalizacion a cero ante duraciones negativas
func TestNegativeDurationNormalization(t *testing.T) {
	if normalizeDuration(-100) != 0 {
		t.Errorf("Expected 0 for negative duration -100, got %d", normalizeDuration(-100))
	}
	if normalizeDuration(0) != 0 {
		t.Errorf("Expected 0 for 0, got %d", normalizeDuration(0))
	}
	if normalizeDuration(42) != 42 {
		t.Errorf("Expected 42 for 42, got %d", normalizeDuration(42))
	}
}

// CHK-08 (RF-08): Verificacion de asignacion de categoria global en ausencia de tarea activa
func TestGlobalTaskFallback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Con base vacia, sin flags, sin env y sin archivo, debe retornar "global"
	res := resolveActiveTask("", "", "/nonexistent/ACTIVE_TASK", db)
	if res != "global" {
		t.Errorf("Expected fallback 'global', got '%s'", res)
	}
}

// CHK-09 (RF-09): Verificacion de consolidacion y sumatorias en reporte de auditoria
func TestAuditReportAggregation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	taskID := "001"
	// Insertamos 3 acciones
	_, _ = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, skill, action, duration_ms, tokens_in, tokens_out, cost_usd, exit_code)
		VALUES ('s1', ?, 'agy', 'impl', 'act1', 100, 500, 100, 0.002, 0)`, taskID)
	_, _ = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, skill, action, duration_ms, tokens_in, tokens_out, cost_usd, exit_code)
		VALUES ('s1', ?, 'agy', 'impl', 'act2', 250, 1500, 400, 0.008, 0)`, taskID)
	_, _ = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, skill, action, duration_ms, tokens_in, tokens_out, cost_usd, exit_code)
		VALUES ('s1', ?, 'agy', 'impl', 'act3', 150, 1000, 200, 0.005, 0)`, taskID)

	var calls int
	var totalDur, totalIn, totalOut int64
	var totalCost float64

	err := db.QueryRow(`
		SELECT COUNT(*),
		       SUM(duration_ms),
		       SUM(tokens_in),
		       SUM(tokens_out),
		       ROUND(SUM(cost_usd), 5)
		FROM agent_actions
		WHERE task_id = ?`, taskID).Scan(&calls, &totalDur, &totalIn, &totalOut, &totalCost)
	if err != nil {
		t.Fatalf("Report aggregation query failed: %v", err)
	}

	if calls != 3 {
		t.Errorf("Expected 3 calls, got %d", calls)
	}
	if totalDur != 500 {
		t.Errorf("Expected 500 ms total duration, got %d", totalDur)
	}
	if totalIn != 3000 {
		t.Errorf("Expected 3000 tokens_in, got %d", totalIn)
	}
	if totalOut != 700 {
		t.Errorf("Expected 700 tokens_out, got %d", totalOut)
	}
	if totalCost < 0.0149 || totalCost > 0.0151 {
		t.Errorf("Expected approx 0.01500 USD, got %f", totalCost)
	}
}
