# Plan y Verificador (FAST) — <nombre_tarea> (Spec <NNN>)

> **Contrato Inmutable (`seb-harness`):** Sellado por `seb plan-lock` y ejecutado por `.seb/bin/seb verify <nombre_tarea>` y `pre-commit`. Prohibido editar durante la implementación.

## 1. Resumen de Cambios y Archivos

- `<ruta/archivo_1>`: [Cambio puntual a realizar]
- `<ruta/archivo_2>`: [Test unitario o validación a agregar/actualizar]

## 2. Matriz de Verificación Determinista

| Check ID | Requisito | Descripción | Comando Determinista |
| :--- | :--- | :--- | :--- |
| <NNN>-CHK-01 | <NNN>-RF-01 | Validación del comportamiento específico | `<comando_test_especifico>` |
| <NNN>-CHK-98 | GLOBAL | Linter / análisis estático oficial (`docs/constitution.md`) | `<comando_linter_oficial>` |
| <NNN>-CHK-99 | GLOBAL | Build y suite oficial de tests (`docs/constitution.md`) | `<comando_build_y_tests_oficial>` |
