---
trigger: always_on
description: "Invariantes de seb-harness: ciclo de vida SDD, principio de menor privilegio, protección anti-reward-hacking y auditoría SQLite."
---

# Regla Global: Gobernanza Determinista seb-harness

Operas bajo el control de **`seb-harness`**. Estas restricciones son acumulativas e innegociables en cada turno:

## 1. Consciencia del Estado y Lectura Previa
- Consulta siempre `@AGENTS.md`, `@docs/constitution.md` y `@MEMORY.md` antes de tomar decisiones técnicas o de diseño.
- Verifica en `@MEMORY.md` la sección **"2. Callejones sin Salida"** para no proponer código ni dependencias previamente descartadas.

## 2.Separación Estricta Generador / Evaluador (Anti-Reward-Hacking)
- Los comandos definidos en `specs/*_verificador.md` actúan como controles computacionales deterministas independientes de tu propio juicio.
- **PROHIBICIÓN ABSOLUTA:** Cuando actúes como agente implementador (escribiendo código de aplicación), tienes terminantemente prohibido modificar, borrar, comentar o relajar las condiciones en:
  - `@docs/constitution.md`
  - `specs/*_spec.md`
  - `specs/*_verificador.md`
- Tampoco puedes alterar tests existentes para forzar que pasen (`skip`, `t.Skip()`, `@Ignore`, `markTestSkipped()`, eliminar assertions o simular retornos falsos). Si un requisito es imposible de cumplir por una limitación real del entorno, detente, regístralo mediante `.seb/bin/seb memory-add` y solicita intervención humana o ejecutar el skill `seb-clarificador`.

## 3. Verificación Programática Obligatoria
- Ninguna tarea se considera terminada por inspección visual o porque "el código parece correcto".
- Todo requisito funcional (`RF-XX`) especificado en `specs/<NNN>_<tarea>_spec.md` debe estar respaldado por al menos un comando ejecutable con código de salida determinista (`0` = éxito) dentro de `specs/<tarea>_verificador.md`.
- La única fuente de verdad para declarar una tarea como completada es la salida del comando `.seb/bin/seb verify <tarea>` ejecutado por el skill `seb-verificador`.

## 4. Trazabilidad y Menor Privilegio
- Respeta estrictamente el alcance del skill activo:
  - Si estás ejecutando `seb-especificador` o `seb-clarificador`, **nunca** modifiques archivos de código fuente ni de configuración del proyecto; limita tu escritura a `specs/<NNN>_<tarea>_spec.md` y `@MEMORY.md`.
  - Si estás ejecutando `seb-planificador`, limita tu escritura a `specs/<tarea>_plan.md`, `specs/<tarea>_verificador.md` y `@MEMORY.md`. Si detectas un solo requisito no verificable programáticamente, aborta la creación del plan.
- Usa las herramientas del binario `.seb/bin/seb` para cualquier operación de secuencia de specs, verificación o reporte de auditoría para garantizar que el rastro quede grabado en `.seb/seb-audit.db`.
