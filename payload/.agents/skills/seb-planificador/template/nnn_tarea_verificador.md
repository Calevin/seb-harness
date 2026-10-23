# Verificador Programático — <nombre_tarea> (Spec <NNN>)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify <nombre_tarea>` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación                      | Comando Determinista                                                |
| :------- | :-------- | :-------------------------------------------------- | :------------------------------------------------------------------ |
| CHK-01   | RF-01     | Test unitario del flujo principal / evento          | `<comando exacto ej: go test -v ./internal/... -run TestRF01>`      |
| CHK-02   | RF-02     | Test de comportamiento no deseado / error (IF/THEN) | `<comando exacto ej: go test -v ./internal/... -run TestRF02Error>` |
| CHK-03   | RF-03     | Test de casos límite (Edge Cases CL-01/CL-02)       | `<comando exacto>`                                                  |
| CHK-98   | RNF-01    | Análisis estático / Linter oficial del proyecto     | `<comando linter de constitution.md>`                               |
| CHK-99   | GLOBAL    | Compilación limpia y suite completa sin regresiones | `<comando build/test general de constitution.md>`                   |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify <nombre_tarea>` reporta **100% de los controles (`CHK-01` a `CHK-99`) en estado `[PASA]`**.
