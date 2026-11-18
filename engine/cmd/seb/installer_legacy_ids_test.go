package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacyNoticeCommand = ".seb/bin/seb role set migrador && .seb/bin/seb oneoff 013 && .seb/bin/seb role clear"

// 013-RF-30
func TestInstallLegacyIdentifiersNotice(t *testing.T) {
	harnessDir := getRepoRootDir(t)

	t.Run("proyecto heredado: aviso con cantidad de tareas y comandos", func(t *testing.T) {
		target := t.TempDir()
		specs := filepath.Join(target, "specs")
		writeSpecStub(t, specs0(t, specs), "001", "a")
		writeVerifierFixture(t, specs, "001", "a", "| CHK-01 | RF-01 | a | `echo a` |\n")
		writeSpecStub(t, specs, "002", "b")
		writeVerifierFixture(t, specs, "002", "b", "| CHK-01 | RF-01 | b | `echo b` |\n")
		writeSpecStub(t, specs, "003", "c")
		writeVerifierFixture(t, specs, "003", "c", "| 003-CHK-01 | 003-RF-01 | c | `echo c` |\n")
		// Verificador con filas heredadas solo dentro de bloque cercado: no cuenta
		writeSpecStub(t, specs, "004", "d")
		writeVerifierFixture(t, specs, "004", "d", "```\n| CHK-01 | RF-01 | ej | `echo` |\n```\n| 004-CHK-01 | 004-RF-01 | d | `echo d` |\n")

		code, stdout, stderr := runInstall(t, harnessDir, "--no-build", target)
		if code != 0 {
			t.Fatalf("install.sh fallo: %d\n%s\n%s", code, stdout, stderr)
		}
		all := stdout + stderr
		if !strings.Contains(all, "AVISO: 2 tareas contienen identificadores heredados") {
			t.Errorf("Se esperaba aviso con 2 tareas heredadas: %s", all)
		}
		if !strings.Contains(all, legacyNoticeCommand) {
			t.Errorf("El aviso debe incluir los comandos exactos de migracion: %s", all)
		}
	})

	t.Run("proyecto ya migrado o sin specs: sin aviso", func(t *testing.T) {
		migrated := t.TempDir()
		specs := filepath.Join(migrated, "specs")
		writeSpecStub(t, specs0(t, specs), "001", "a")
		writeVerifierFixture(t, specs, "001", "a", "| 001-CHK-01 | 001-RF-01 | a | `echo a` |\n")
		for _, target := range []string{migrated, t.TempDir()} {
			code, stdout, stderr := runInstall(t, harnessDir, "--no-build", target)
			if code != 0 {
				t.Fatalf("install.sh fallo: %d\n%s\n%s", code, stdout, stderr)
			}
			if strings.Contains(stdout+stderr, "identificadores heredados") {
				t.Errorf("No debe avisar en proyectos sin heredados: %s", stdout+stderr)
			}
		}
	})
}

// specs0 crea el directorio de specs y lo devuelve.
func specs0(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	return dir
}
