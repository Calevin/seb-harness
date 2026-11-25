---
trigger: always_on
description: "Invariantes universales de seb-harness: fuentes de verdad, integridad de tests y sujeción a seb guard."
---

# Regla Global: Invariantes de Gobernanza (`seb-harness`)

Operas bajo **`seb-harness`**. Estas restricciones aplican en todo turno:

## 1. Fuentes de Verdad y Contexto
- Consulta `@AGENTS.md`, `@docs/constitution.md` y `@MEMORY.md` antes de proponer cambios técnicos o de diseño.
- No reintentes enfoques documentados en la sección **"2. Callejones sin Salida"** de `@MEMORY.md`.

## 2. Integridad y Anti-Reward-Hacking
- Como implementador, tienes **terminantemente prohibido** modificar, comentar o relajar `@docs/constitution.md`, `specs/*_spec.md` o `specs/*_verificador.md`.
- Prohibido deshabilitar tests (`skip`, `t.Skip()`, `@Ignore`, `markTestSkipped()`), eliminar aserciones o simular retornos falsos para forzar un aprobado. Si un requisito es inviable, detente, regístralo con `.seb/bin/seb memory-add` y solicita `seb-clarificador`.

## 3. Menor Privilegio y Protocolo SDD
- Respeta estrictamente el alcance del rol activo impuesto por `.seb/bin/seb guard` y nunca escales al rol `migrador` ni ejecutes `seb oneoff`.
- Para tareas bajo flujo SDD (especificar, clarificar, planificar, implementar, verificar o documentar), lee y sigue el protocolo completo en `.agents/SEB-HARNESS.md`.

## 4. Integridad del Grafo de Dependencias (DAG)
- Prohibido planificar o implementar una tarea cuyas dependencias directas no estén verificadas (`verified: true` y `commit_hash` en `.seb/seb.lock`).
