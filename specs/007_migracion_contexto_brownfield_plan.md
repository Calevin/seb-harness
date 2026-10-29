# Plan Técnico — Asimilación de Contexto Preexistente en Proyectos Brownfield (Spec 007)

- **Spec Origen:** `specs/007_migracion_contexto_brownfield_spec.md`
- **Verificador Asociado:** `specs/007_migracion_contexto_brownfield_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

La adopción de `seb-harness` en proyectos existentes (brownfield) requiere gestionar con seguridad las instrucciones y directrices de asistentes de inteligencia artificial previamente configuradas por el equipo de desarrollo (`AGENTS.md`, `GEMINI.md`, `CLAUDE.md`, `docs/constitution.md`). Si el arnés ignora estos archivos o los sobrescribe a ciegas, se produce pérdida de conocimiento tácito o divergencia de directivas (*split-brain*).

Para solucionar este desafío respetando los principios de gobernanza determinista, menor privilegio y reversibilidad completa, se implementa una arquitectura basada en cuatro pilares:

1. **Reconocimiento Determinista de Pertenencia (Firma Canónica):**
   Una función auxiliar `has_seb_signature` evaluará la presencia de la subcadena `seb-harness` de forma insensible a mayúsculas y minúsculas (`grep -qi "seb-harness"`). Cualquier archivo de contexto que carezca de dicha firma es catalogado formalmente como ajeno/preexistente.
2. **Respaldo No Destructivo con Infijo Canónico (`.pre-seb`):**
   Durante la ejecución de `install.sh`:
   - Si `AGENTS.md` existe y no contiene la firma, se respalda como `AGENTS.pre-seb.md` antes de copiar la plantilla oficial del protocolo operativo de `payload/AGENTS.md`.
   - Si existen archivos de asistentes de IA como `GEMINI.md` o `CLAUDE.md` sin la firma del arnés, se respaldan automáticamente bajo la nomenclatura `<nombre>.pre-seb.<ext>` (ej. `GEMINI.pre-seb.md`).
   - Se implementa un guardarraíl anti-sobrescritura: si el archivo de respaldo ya existe en disco, se preserva intacto y se emite una advertencia al usuario.
3. **Reconocimiento y Asimilación en `seb-iniciador` (Anti-Split-Brain):**
   Se actualizan los guardarraíles e instrucciones del skill `seb-iniciador`:
   - En la Fase 1, se incluyen expresamente los archivos `AGENTS.pre-seb.md`, `GEMINI.pre-seb.md`, `CLAUDE.pre-seb.md` y `docs/constitution.pre-seb.md` en la lista de inspección prioritaria.
   - Si `docs/constitution.md` ya existe en el proyecto destino sin la firma de `seb-harness`, se respalda a `docs/constitution.pre-seb.md` y se asimila su contenido. Si ya cuenta con la firma canónica, se protege contra sobrescritura.
   - Tras consolidar la constitución, `seb-iniciador` genera o actualiza los archivos de asistentes (`GEMINI.md`, `CLAUDE.md`) como stubs canónicos ligeros que redirigen formalmente a `docs/constitution.md` y `AGENTS.md`.
4. **Reversibilidad Completa en Desinstalación (`uninstall.sh`):**
   Durante la ejecución de `uninstall.sh`, el script buscará todos los archivos con patrón `*.pre-seb.*` generados en el proyecto y los restaurará a su denominación original sin el infijo `.pre-seb`, soportando además modo `--dry-run`.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica | Requisitos Cubiertos (`RF-XX`) |
| :--- | :--- | :--- | :--- |
| `install.sh` | `MODIFICAR` | Incorporar detección de firma `seb-harness`, respaldo de `AGENTS.md` a `AGENTS.pre-seb.md`, respaldo de `GEMINI.md` y `CLAUDE.md` a `<nombre>.pre-seb.<ext>`, y protección anti-sobrescritura de backups. | `RF-01`, `RF-02`, `RF-07`, `RF-09`, `RNF-01`, `RNF-02`, `CL-01`, `CL-02` |
| `uninstall.sh` | `MODIFICAR` | Incorporar detección y restauración automática de archivos `*.pre-seb.*` a sus nombres originales (con soporte de `--dry-run`). | `RF-08`, `RNF-01`, `RNF-02` |
| `payload/.agents/skills/seb-iniciador/SKILL.md` | `MODIFICAR` | Actualizar permisos de escritura para stubs, lista de inspección de contexto (incluyendo archivos `.pre-seb`), protocolo de constitución previa y directiva de generación de stubs anti-split-brain. | `RF-03`, `RF-04`, `RF-05`, `RF-06`, `CL-03` |
| `engine/cmd/seb/brownfield_migration_test.go` | `CREAR` | Suite determinista de pruebas unitarias y de integración en Go que valida respaldos de instalación, stubs, restauración en desinstalación y casos límite. | `RF-01` a `RF-09`, `RNF-01`, `RNF-02`, `CL-01`, `CL-02` |

## 3. Contratos, Firmas y Formato de Artefactos

### 3.1 Detección de Firma en Bash (`install.sh`)

```bash
has_seb_signature() {
    local file="$1"
    [ -f "$file" ] && grep -qi "seb-harness" "$file"
}

backup_pre_seb() {
    local file="$1"
    [ -f "$file" ] || return 0
    if ! has_seb_signature "$file"; then
        local dir base name ext bak
        dir="$(dirname "$file")"
        base="$(basename "$file")"
        if [[ "$base" == *.* ]]; then
            name="${base%.*}"
            ext="${base##*.}"
            bak="$dir/$name.pre-seb.$ext"
        else
            bak="$dir/$base.pre-seb"
        fi

        if [ -e "$bak" ]; then
            warn "Ya existe un respaldo $bak; se conserva el respaldo previo."
        else
            mv "$file" "$bak"
            info "Detectado archivo ajeno a seb-harness. Respaldado: $base -> $(basename "$bak")"
        fi
    fi
}
```

### 3.2 Formato del Stub de Redirección Canónico (`GEMINI.md` / `CLAUDE.md`)

```markdown
# Contexto del Proyecto

> Este proyecto implementa gobernanza y desarrollo determinista bajo **seb-harness**.
> Todas las directrices de arquitectura, stack y estilo están consolidadas en la constitución del proyecto.

- **Ley Suprema de Arquitectura y Dominio:** [docs/constitution.md](docs/constitution.md)
- **Protocolo Operativo de Agentes:** [AGENTS.md](AGENTS.md)
- **Bitácora y Memoria Técnica:** [MEMORY.md](MEMORY.md)
```

## 4. Tareas Atómicas de Implementación (TDD)

- **Tarea 1 (TDD - Test Suite Base):** Crear `engine/cmd/seb/brownfield_migration_test.go` con los tests de respaldo de `AGENTS.md`, asistentes externos, restauración en `uninstall.sh` y casos límite. Confirmar fallo inicial esperado (fase roja).
- **Tarea 2 (Script de Instalación):** Modificar `install.sh` implementando `has_seb_signature` y `backup_pre_seb`. Respaldar `AGENTS.md` si carece de firma e instalar la plantilla oficial; respaldar `GEMINI.md` y `CLAUDE.md` sin firma preservando backups previos si existen.
- **Tarea 3 (Script de Desinstalación):** Modificar `uninstall.sh` para buscar y restaurar quirúrgicamente todos los archivos `*.pre-seb.*` a su denominación original (respetando `--dry-run`).
- **Tarea 4 (Skill seb-iniciador):** Actualizar `payload/.agents/skills/seb-iniciador/SKILL.md` (y su enlace simbólico) incorporando la lectura de archivos `.pre-seb`, la regla de respaldo de `docs/constitution.md` previa y la plantilla de stubs anti-split-brain.
- **Tarea 5 (Verificación y Suite Completa):** Ejecutar `go test`, `go vet`, compilación de `seb` y verificación determinista completa mediante `.seb/bin/seb verify migracion_contexto_brownfield`.

## 5. Criterios de Aceptación y Guardarraíles de Verificación

- `CHK-01` a `CHK-05` en estado `[PASA]`.
- Cero advertencias en `go vet ./...`.
- Cero violaciones a `docs/constitution.md`.
