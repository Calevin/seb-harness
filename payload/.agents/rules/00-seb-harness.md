---
trigger: always_on
description: "Invariantes de seb-harness: ciclo de vida SDD, principio de menor privilegio, protección anti-reward-hacking, auditoría SQLite e integridad de dependencias DAG."
---

# Regla Global: Gobernanza Determinista seb-harness

Operas bajo el control de **`seb-harness`**. Estas restricciones son acumulativas e innegociables en cada turno:

## 1. Consciencia del Estado y Lectura Previa
- Consulta siempre `@AGENTS.md`, `@docs/constitution.md` y `@MEMORY.md` antes de tomar decisiones técnicas o de diseño.
- Verifica en `@MEMORY.md` la sección **"2. Callejones sin Salida"** para no proponer código ni dependencias previamente descartadas.

## 2. Separación Estricta Generador / Evaluador (Anti-Reward-Hacking)
- Los comandos definidos en `specs/*_verificador.md` actúan como controles computacionales deterministas independientes de tu propio juicio.
- **PROHIBICIÓN ABSOLUTA:** Cuando actúes como agente implementador (escribiendo código de aplicación), tienes terminantemente prohibido modificar, borrar, comentar o relajar las condiciones en:
  - `@docs/constitution.md`
  - `specs/*_spec.md`
  - `specs/*_verificador.md`
- Tampoco puedes alterar tests existentes para forzar que pasen (`skip`, `t.Skip()`, `@Ignore`, `markTestSkipped()`, eliminar assertions o simular retornos falsos). Si un requisito es imposible de cumplir por una limitación real del entorno, detente, regístralo mediante `.seb/bin/seb memory-add` y solicita intervención humana o ejecutar el skill `seb-clarificador`.

## 3. Verificación Programática Obligatoria
- Ninguna tarea se considera terminada por inspección visual o porque "el código parece correcto".
- Todo requisito funcional (`RF-XX`) especificado en `specs/<NNN>_<tarea>_spec.md` debe estar respaldado por al menos un comando ejecutable con código de salida determinista (`0` = éxito) dentro de `specs/<NNN>_<tarea>_verificador.md`.
- La única fuente de verdad para declarar una tarea como completada es la salida del comando `.seb/bin/seb verify <tarea>` ejecutado por el skill `seb-verificador`.

## 4. Trazabilidad y Menor Privilegio
- Respeta estrictamente el alcance del skill activo:
  - Si estás ejecutando `seb-especificador` o `seb-clarificador`, **nunca** modifiques archivos de código fuente ni de configuración del proyecto; limita tu escritura a `specs/<NNN>_<tarea>_spec.md` y `@MEMORY.md`.
  - Si estás ejecutando `seb-planificador`, limita tu escritura a `specs/<NNN>_<tarea>_plan.md`, `specs/<NNN>_<tarea>_verificador.md` y `@MEMORY.md`. Si detectas un solo requisito no verificable programáticamente o dependencias insatisfechas en el DAG, aborta la creación del plan.
  - Si estás ejecutando `seb-documentador`, limita tu escritura exclusivamente a `docs/use-cases/*.md`, `docs/use-cases/README.md` y `@MEMORY.md`. Tienes terminantemente prohibido modificar código fuente, tests, la constitución o especificaciones cerradas.
- Usa las herramientas del binario `.seb/bin/seb` para cualquier operación de secuencia de specs, verificación o reporte de auditoría para garantizar que el rastro quede grabado en `.seb/seb-audit.db`.

## 5. Integridad del Grafo de Dependencias (DAG)
- Toda tarea que declare dependencias en su spec (`Depends-On:` / `Dependencias:`) requiere que dichas dependencias directas estén formalmente verificadas (`verified: true` y `commit_hash` no vacío en `.seb/seb.lock`).
- Queda terminantemente prohibido planificar o implementar código sobre una tarea si sus prerrequisitos directos no han superado exitosamente su propia suite de verificación determinista.

## 6. Identificadores Únicos de Requisitos
- Todo elemento asociado a una spec (`HU`, `RF`, `RNF`, `CL`, `CHK`) usa el identificador prefijado `<NNN>-<TIPO>-<NN>` (ej. `013-RF-05`); el formato sin prefijo es heredado y se rechaza en los verificadores.
- Al referenciar un requisito fuera de su propia spec (comentarios de código, tests, `MEMORY.md`, commits, casos de uso) usa siempre el identificador prefijado, solo en texto (comentarios y documentos), nunca como parte de identificadores de código.

## 7. Operaciones One-Off de Actualización del Arnés
- Las Operaciones One-Off son excepcionales, están identificadas por el ID de la spec que las define y se ejecutan con `.seb/bin/seb oneoff <id_spec>`.
- Solo puede ejecutarlas el humano con el rol `migrador` activo; **tienes prohibido** activar ese rol (`seb role set migrador`, `SEB_SKILL=migrador`) o ejecutar una one-off: el hook lo deniega. Si detectas que hace falta una, indícale al humano el comando exacto.
- Son la única excepción autorizada a la inmutabilidad de specs y verificadores sellados (reescritura y resellado auditados en `.seb/seb.lock` y `.seb/seb-audit.db`).
