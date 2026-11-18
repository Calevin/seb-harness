# Verificador Programático — Soporte de Múltiples Casos de Uso en el Metadato `Afecta-UC:` (Spec 014)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify afecta_uc_multiple` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados. La única adaptación de tests permitida es la acotada por `RF-14` y está custodiada por `CHK-05`.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01,RF-02,RF-03,RF-04,RF-13,CL-01 | Directiva y plantilla de seb-especificador: literales canonicos de lista, nuevo unico, ninguno exclusivo, criterio cohesivo/independiente y ausencia de la regla de division obligatoria | `direnv exec . bash -c "cd engine && go test -v -run TestEspecificadorAfectaUCList ./cmd/seb"` |
| CHK-02 | RF-05,RF-06,RF-07,CL-02,CL-03,CL-04,CL-05,CL-06 | Directiva de seb-clarificador: validacion de sintaxis y existencia por elemento, orden irrelevante, respaldo por RF-XX, cobertura inversa y Hard Gate para listas invalidas | `direnv exec . bash -c "cd engine && go test -v -run TestClarificadorAfectaUCList ./cmd/seb"` |
| CHK-03 | RF-08,RF-09,RF-10,RF-11,RF-12,CL-07 | Directiva de seb-documentador: actualizacion por elemento, resolucion de nuevo, columna Specs de Evolucion, migracion con guion y no duplicacion | `direnv exec . bash -c "cd engine && go test -v -run TestDocumentadorAfectaUCList ./cmd/seb"` |
| CHK-04 | RF-14,RF-15 | Suite de la Spec 011 (CHK-01 a CHK-03 originales) sigue pasando tras la adaptacion acotada | `direnv exec . bash -c "cd engine && go test -v -run 'TestEspecificadorAfectaUCDirectives|TestClarificadorAfectaUCDirectives|TestAfectaUCBackwardCompatibilityAndSync' ./cmd/seb"` |
| CHK-05 | RF-14 | Guardarrail anti-relajacion del test adaptado de la Spec 011: aserciones criticas conservadas, literales nuevos presentes, sin t.Skip, sin literal sustituido y al menos 17 aserciones | `bash -c 'f=engine/cmd/seb/afecta_uc_governance_test.go; grep -qF "without editing the spec beforehand" $f && grep -qF "STRICTLY FORBIDDEN from marking a spec as" $f && grep -qF "010_integracion_documentador" $f && grep -qF "Desincronizacion entre" $f && grep -qF "cohesive" $f && grep -qF "lista:" $f && ! grep -qE "t\.Skip|more than one independent" $f && test $(grep -cE "t\.(Errorf|Fatalf)" $f) -ge 17'` |
| CHK-06 | RF-15,RF-17 | Integridad del sello de la Spec 011: SHA-256 en disco de spec y verificador coinciden con los valores sellados | `bash -c 'test "$(sha256sum specs/011_gobernanza_afecta_uc_spec.md | cut -c1-64)" = e26bd60609e85f4e62cf7c26e7e3a9dbaa82162a6e3757a1bd3c7c055ff76012 && test "$(sha256sum specs/011_gobernanza_afecta_uc_verificador.md | cut -c1-64)" = d1260ca4951fc5596265acdc32169fbe5c0456927be9e7d22b86c7a96a924b10'` |
| CHK-07 | RF-16,RF-17,RNF-01 | Distribucion via install.sh a .agents y .claude del destino sin emojis, y compatibilidad de specs previas con valor simple en seb.lock y en disco | `direnv exec . bash -c "cd engine && go test -v -run 'TestAfectaUCListDistribution|TestAfectaUCListBackwardCompatibility' ./cmd/seb"` |
| CHK-08 | RF-16 | El motor Go no incorpora logica de parseo de Afecta-UC fuera de archivos de test | `bash -c '! grep -rl "Afecta-UC" engine --include=*.go | grep -v _test.go'` |
| CHK-98 | RNF-02 | Analisis estatico oficial del proyecto (go vet) sin advertencias | `direnv exec . bash -c "cd engine && go vet ./..."` |
| CHK-99 | GLOBAL,RNF-02 | Compilacion limpia de binario seb y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify afecta_uc_multiple` (o `014`) reporta **100% de los controles (`CHK-01` a `CHK-99`) en estado `[PASA]`**.
- `.seb/bin/seb verify gobernanza_afecta_uc` debe continuar en `[PASA]` (cubierto de forma equivalente por `CHK-04`, `CHK-06`, `CHK-98` y `CHK-99`).
