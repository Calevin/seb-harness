# Plan Técnico — Integridad en Reserva Secuencial y Purga de Tareas (Spec 015)

- **Spec Origen:** `specs/015_integridad_reserva_purga_spec.md`
- **Verificador Asociado:** `specs/015_integridad_reserva_purga_verificador.md`
- **Estado:** `PLANIFICADO`
- **Dependencias verificadas:** Ninguna declarada.

## 1. Resumen Arquitectónico y Decisiones de Diseño

La intervención se concentra en el motor CLI en Go (`engine/cmd/seb/main.go`), su suite de tests unitarios/integración y la directiva del skill `seb-especificador`:

1. **Reserva secuencial atómica (`specNext`):** Se extrae la lógica de `cmdSpecNext` a una función retornante de error `specNext(db *sql.DB, args []string) (string, error)` para habilitar pruebas unitarias directas sin `os.Exit`. El cálculo secuencial toma el máximo entre los prefijos en `specs/` y las filas en `tasks` (manteniendo autonomía multi-máquina en clones limpios sin DB local), valida nombres inválidos o con traversal, soporta `--dry-run`, implementa idempotencia acotada cuando el archivo existente coincide exactamente con el stub inicial, rechaza nombres duplicados sobre specs ya redactadas y rechaza cualquier colisión de ID con nombre distinto eliminando el `ON CONFLICT(id) DO UPDATE`.
2. **Materialización atómica y rollback:** La inserción en `tasks` se envuelve en una transacción SQLite (`db.Begin()`) que solo se confirma (`tx.Commit()`) tras crear y cerrar exitosamente en disco el archivo stub mediante `os.OpenFile(..., os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)`. Ante cualquier fallo de escritura, `tx.Rollback()` revierte la reserva local y limpia residuos parciales.
3. **Purga acotada a identidad unívoca (`executeTaskPurge`):** Se elimina el predicado inseguro `OR name = ?` en `DELETE FROM tasks` (restringido a `WHERE id = ?`) y se acota la limpieza en `verifications` y `agent_actions` exclusivamente a `task.ID` y `task.CanonicalName` (nunca `task.Name`).
4. **Actualización documental del skill:** Se actualiza `payload/.agents/skills/seb-especificador/SKILL.md` (Fases 3 y 4) para reflejar que `seb spec-next` crea el archivo stub inicial en disco que el agente debe completar/sobrescribir.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción | Responsabilidad Técnica | Requisitos Cubiertos |
| :--- | :--- | :--- | :--- |
| `engine/cmd/seb/main.go` | `MODIFICAR` | Refactor de `cmdSpecNext` -> `specNext`, `buildSpecStubContent`, creación atómica con `O_EXCL` y rollback SQLite, `--dry-run`, validación de nombres y colisiones; purga acotada en `executeTaskPurge` | `015-RF-01`..`015-RF-10`, `015-RNF-01`, `015-RNF-02`, `015-CL-01`..`015-CL-05` |
| `payload/.agents/skills/seb-especificador/SKILL.md` | `MODIFICAR` | Documentar en Fases 3 y 4 que `seb spec-next` crea el archivo stub inicial y el agente lo completa/sobrescribe | `015-RF-11` |
| `engine/cmd/seb/spec_next_purge_integrity_test.go` | `CREAR` | Tests unitarios e integración para reservas encadenadas, colisión de ID, idempotencia de stub vs rechazo de spec editada, `--dry-run`, rollback por error de disco, nombres inválidos, purga unívoca con nombres duplicados y directiva del skill | `015-RF-01`..`015-RF-11`, `015-RNF-01`, `015-RNF-02`, `015-CL-01`..`015-CL-05` |

## 3. Contratos de Implementación

### 3.1 Plantilla del Archivo Stub Inicial (`buildSpecStubContent`)

```markdown
# Spec <NNN> — <nombre_tarea>

- **Tarea ID:** `<NNN>`
- **Identificador:** `<nombre_tarea>`
- **Origen:** [Pendiente]
- **Estado:** `BORRADOR`
- **Afecta-UC:** [PENDIENTE]

## 10. Dudas Abiertas y Puntos por Clarificar

- [NECESITA ACLARACIÓN] Stub recién reservado por `seb spec-next`; pendiente redactar especificación completa.
```

La comparación de idempotencia (`015-RF-04`, `015-RF-05`) normaliza `\r\n` a `\n` y verifica igualdad exacta contra `buildSpecStubContent(id, taskName)`.

### 3.2 Reglas de `executeTaskPurge`

- `DELETE FROM verifications WHERE task_id = ? OR task_id = ?` con `(task.ID, task.CanonicalName)`
- `DELETE FROM agent_actions WHERE task_id = ? OR task_id = ?` con `(task.ID, task.CanonicalName)`
- `DELETE FROM tasks WHERE id = ?` con `(task.ID)`
- En `seb.lock`: eliminar `lock.Tasks[task.CanonicalName]` (preservando otras entradas aunque compartan nombre simple).

## 4. Desglose de Tareas Atómicas (TDD)

1. **Tarea 1 (Tests primero — `spec_next_purge_integrity_test.go`):** Escribir suite de tests cubriendo `TestSpecNextSequentialChaining`, `TestSpecNextCollisionRejection`, `TestSpecNextStubIdempotencyAndEditedRejection`, `TestSpecNextDryRunAndInvalidNames`, `TestSpecNextDiskFailureRollback`, `TestAbortPurgeScopedToUniqueID` y `TestEspecificadorStubDirective`.
2. **Tarea 2 (Motor `specNext`):** Implementar `buildSpecStubContent`, validación estricta de nombres, parseo de `--dry-run`, cálculo `max(specs/, tasks)`, detección de stub vs spec editada, rechazo de colisión de ID sin `ON CONFLICT DO UPDATE`, apertura `O_CREATE|O_EXCL` y rollback transaccional en SQLite.
3. **Tarea 3 (Motor `executeTaskPurge`):** Acotar las consultas `DELETE` en `tasks`, `verifications` y `agent_actions` exclusivamente a `task.ID` / `task.CanonicalName`.
4. **Tarea 4 (Skill `seb-especificador`):** Actualizar `payload/.agents/skills/seb-especificador/SKILL.md` indicando la creación previa del stub inicial por `spec-next` y su sobrescritura/completado en Fase 4.
5. **Tarea 5 (Build & Verificación):** Compilar `.seb/bin/seb` con `CGO_ENABLED=0`, ejecutar `go vet ./...` y toda la suite `go test -v ./...`.
