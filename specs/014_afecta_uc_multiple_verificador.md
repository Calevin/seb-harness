# Verificador Programático — Soporte de Múltiples Casos de Uso en el Metadato `Afecta-UC:` (Spec 014)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify afecta_uc_multiple` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados. La única adaptación de tests permitida es la acotada por `014-RF-14` y está custodiada por `014-CHK-05`.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| 014-CHK-01 | 014-RF-01,014-RF-02,014-RF-03,014-RF-04,014-RF-13,014-CL-01 | Directiva y plantilla de seb-especificador: literales canonicos de lista, nuevo unico, ninguno exclusivo, criterio cohesivo/independiente y ausencia de la regla de division obligatoria | `direnv exec . bash -c "cd engine && go test -v -run TestEspecificadorAfectaUCList ./cmd/seb"` |
| 014-CHK-02 | 014-RF-05,014-RF-06,014-RF-07,014-CL-02,014-CL-03,014-CL-04,014-CL-05,014-CL-06 | Directiva de seb-clarificador: validacion de sintaxis y existencia por elemento, orden irrelevante, respaldo por RF-XX, cobertura inversa y Hard Gate para listas invalidas | `direnv exec . bash -c "cd engine && go test -v -run TestClarificadorAfectaUCList ./cmd/seb"` |
| 014-CHK-03 | 014-RF-08,014-RF-09,014-RF-10,014-RF-11,014-RF-12,014-CL-07 | Directiva de seb-documentador: actualizacion por elemento, resolucion de nuevo, columna Specs de Evolucion, migracion con guion y no duplicacion | `direnv exec . bash -c "cd engine && go test -v -run TestDocumentadorAfectaUCList ./cmd/seb"` |
| 014-CHK-04 | 014-RF-14,014-RF-15 | Suite de la Spec 011 (CHK-01 a CHK-03 originales) sigue pasando tras la adaptacion acotada | `direnv exec . bash -c "cd engine && go test -v -run 'TestEspecificadorAfectaUCDirectives|TestClarificadorAfectaUCDirectives|TestAfectaUCBackwardCompatibilityAndSync' ./cmd/seb"` |
| 014-CHK-05 | 014-RF-14 | Guardarrail anti-relajacion del test adaptado de la Spec 011: aserciones criticas conservadas, literales nuevos presentes, sin t.Skip, sin literal sustituido y al menos 17 aserciones | `bash -c 'f=engine/cmd/seb/afecta_uc_governance_test.go; grep -qF "without editing the spec beforehand" $f && grep -qF "STRICTLY FORBIDDEN from marking a spec as" $f && grep -qF "010_integracion_documentador" $f && grep -qF "Desincronizacion entre" $f && grep -qF "cohesive" $f && grep -qF "lista:" $f && ! grep -qE "t\.Skip|more than one independent" $f && test $(grep -cE "t\.(Errorf|Fatalf)" $f) -ge 17'` |
| 014-CHK-06 | 014-RF-15,014-RF-17 | Integridad del sello de la Spec 011: SHA-256 en disco de spec y verificador coinciden con los valores sellados | `bash -c 'test "$(sha256sum specs/011_gobernanza_afecta_uc_spec.md | cut -c1-64)" = 825b1112450685449a0cfb9516b970055c649c4d935f747000a73517118ce8f1 && test "$(sha256sum specs/011_gobernanza_afecta_uc_verificador.md | cut -c1-64)" = a457feadac396ccedf3a3b9740f22e98e6c087bfdc9e0d595c951c921ee27cbd'` |
| 014-CHK-07 | 014-RF-16,014-RF-17,014-RNF-01 | Distribucion via install.sh a .agents y .claude del destino sin emojis, y compatibilidad de specs previas con valor simple en seb.lock y en disco | `direnv exec . bash -c "cd engine && go test -v -run 'TestAfectaUCListDistribution|TestAfectaUCListBackwardCompatibility' ./cmd/seb"` |
| 014-CHK-08 | 014-RF-16 | El motor Go no incorpora logica de parseo de Afecta-UC fuera de archivos de test | `bash -c '! grep -rl "Afecta-UC" engine --include=*.go | grep -v _test.go'` |
| 014-CHK-98 | 014-RNF-02 | Analisis estatico oficial del proyecto (go vet) sin advertencias | `direnv exec . bash -c "cd engine && go vet ./..."` |
| 014-CHK-99 | GLOBAL,014-RNF-02 | Compilacion limpia de binario seb y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify afecta_uc_multiple` (o `014`) reporta **100% de los controles (`014-CHK-01` a `014-CHK-99`) en estado `[PASA]`**.
- `.seb/bin/seb verify gobernanza_afecta_uc` debe continuar en `[PASA]` (cubierto de forma equivalente por `014-CHK-04`, `014-CHK-06`, `014-CHK-98` y `014-CHK-99`).
