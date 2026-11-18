# Verificador Programático — <nombre_tarea> (Spec <NNN>)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify <nombre_tarea>` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID     | Requisito                  | Descripción de la Verificación                      | Comando Determinista                                                |
| :----------- | :------------------------- | :-------------------------------------------------- | :------------------------------------------------------------------ |
| <NNN>-CHK-01 | <NNN>-RF-01 | Test unitario del flujo principal / evento          | `<comando exacto ej: go test -v ./internal/... -run TestMainFlow>`  |
| <NNN>-CHK-02 | <NNN>-RF-02 | Test de comportamiento no deseado / error (IF/THEN) | `<comando exacto ej: go test -v ./internal/... -run TestErrorCase>` |
| <NNN>-CHK-03 | <NNN>-RF-03, <NNN>-CL-01   | Test de casos límite (Edge Cases)                   | `<comando exacto>`                                                  |
| <NNN>-CHK-98 | <NNN>-RNF-01 | Análisis estático / Linter oficial del proyecto     | `<comando linter de constitution.md>`                               |
| <NNN>-CHK-99 | GLOBAL                     | Compilación limpia y suite completa sin regresiones | `<comando build/test general de constitution.md>`                   |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify <nombre_tarea>` reporta **100% de los controles (`<NNN>-CHK-01` a `<NNN>-CHK-99`) en estado `[PASA]`**.
