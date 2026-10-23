# Plan Técnico — Desinstalación y Reversión del Arnés (Spec 003)

- **Spec Origen:** `specs/003_desinstalacion_arnes_spec.md`
- **Verificador Asociado:** `specs/003_desinstalacion_arnes_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

El script `install.sh` opera como aprovisionador copiando componentes desde `payload/` a proyectos destino (o creando un symlink en modo dogfooding `--self`). Para revertir estas operaciones de forma determinista y segura, se implementará `uninstall.sh` en la raíz del repositorio, siguiendo exactamente la misma arquitectura y filosofía de diseño que `install.sh`:

1. **Aislamiento en Bash portable (`set -euo pipefail`):**
   `uninstall.sh` no requiere dependencias pesadas ni compiladores locales para ejecutarse. Al ejecutarse directamente con utilidades estándar de Bash, Git y Coreutils, puede limpiar un repositorio destino e incluso eliminar el binario `.seb/bin/seb` y la base `.seb/seb-audit.db` sin conflictos de bloqueo de archivos en ejecución.

2. **Doble modalidad operativa:**
   - **Modo Proyecto Destino (Target Project):** Opera sobre un repositorio externo `$TARGET` (o `$PWD`). Desvincula hooks de Git del arnés, restaura respaldos `.bak-seb`, limpia de forma quirúrgica `.gitignore` (solamente las entradas agregadas por el instalador), elimina `.seb/`, y remueve únicamente los archivos/carpetas en `.agents/` que coinciden con `payload/.agents/`, preservando extensiones locales del desarrollador.
   - **Modo Dogfooding (`--self`):** Detecta o exige que se ejecute en el propio repositorio `seb-harness`, removiendo únicamente el symlink `.agents` y los hooks de desarrollo, garantizando la inmunidad de `payload/` y `engine/`.

3. **Salvaguarda de Datos de Dominio y Confirmación:**
   Por defecto, `uninstall.sh` preserva `specs/`, `docs/constitution.md`, `AGENTS.md` y `MEMORY.md`. La eliminación de estos artefactos queda confinada a la bandera `--purge`. Para evitar catástrofes accidentales, `--purge` exige confirmación interactiva en terminal (`[y/N]`), abortando con código de salida `1` en entornos no interactivos (como CI) salvo que se pase explícitamente `--force` (o `-f`). Además, la limpieza de `docs/` elimina `constitution.md` y solo borra la carpeta `docs/` si queda vacía.

4. **Suite de Integración Determinista en Go (`engine/cmd/seb/uninstall_test.go`):**
   Para garantizar la verificación programática automática según `docs/constitution.md` (Sección B), la totalidad de las ramas, opciones y casos límite se verificarán mediante una suite de tests en Go que prepara entornos temporales aislados (`t.TempDir()`), inicializa repositorios Git de prueba, ejecuta `install.sh` y `uninstall.sh`, y valida exhaustivamente el sistema de archivos resultante y los códigos de salida.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta                     | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica                                                                                                                          | Requisitos Cubiertos (`RF-XX`)           |
| :--------------------------------- | :----------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------- | :--------------------------------------- |
| `uninstall.sh`                     | `CREAR`                        | Script principal de desinstalación con interfaz CLI, flags (`--self`, `--dry-run`, `--purge`, `--force`, `-h`), validaciones y limpieza atómica. | `RF-01` a `RF-18`, `RNF-01`, `RNF-02`    |
| `engine/cmd/seb/uninstall_test.go` | `CREAR`                        | Suite de tests de integración en Go que valida programáticamente todas las aserciones de desinstalación con fixtures reales.                     | `RF-01` a `RF-18`, `RNF-01`, `RNF-02`    |
| `README.md`                        | `MODIFICAR`                    | Documentación técnica de referencia explicando el uso de `uninstall.sh` junto a `install.sh`.                                                    | Documentación y consistencia de proyecto |

## 3. Contratos, Firmas y Modelo de Datos

### Interfaz de Línea de Comandos (`uninstall.sh`)

```bash
Uso:
  ./uninstall.sh [opciones] [DIRECTORIO_DESTINO]    (por defecto: directorio actual)

Opciones:
  --self       Revertir dogfooding en este repositorio (remueve symlink .agents y hooks de git).
  --dry-run    Simular la desinstalación mostrando qué archivos y hooks se modificarían sin aplicar cambios.
  --purge      Eliminar adicionalmente specs/, AGENTS.md, MEMORY.md y docs/constitution.md.
  -f, --force  Omitir confirmación interactiva al usar --purge (mandatorio en entornos CI / no-TTY).
  -h, --help   Mostrar este mensaje de ayuda.
```

### Códigos de Salida

- `0`: Desinstalación exitosa, simulación completada o despliegue de ayuda.
- `1`: Error de validación (directorio no encontrado, target inválido con `--self`, `--purge` sin TTY ni `--force`, o flags incompatibles).

### Pseudocódigo de Algoritmos Críticos

#### 1. Detección de Hooks del Arnés y Restauración de Respaldos

```bash
revert_hook() {
    local hooks_dir="$1" name="$2"
    local dest="$hooks_dir/$name"
    local bak="$hooks_dir/$name.bak-seb"

    if [ -L "$dest" ]; then
        local target
        target="$(readlink "$dest")"
        if [[ "$target" == *".agents/hooks/"* ]]; then
            rm -f "$dest"
            if [ -f "$bak" ]; then
                mv "$bak" "$dest"
            fi
        fi
    fi
}
```

#### 2. Limpieza Quirúrgica de `.gitignore`

```bash
clean_gitignore() {
    local gitignore="$1/.gitignore"
    [ -f "$gitignore" ] || return 0
    local tmp
    tmp="$(mktemp)"
    grep -vxF -e ".seb/bin/" -e ".seb/seb-audit.db*" "$gitignore" > "$tmp" || true
    if [ -s "$tmp" ]; then
        mv "$tmp" "$gitignore"
    else
        rm -f "$tmp" "$gitignore"
    fi
}
```

#### 3. Limpieza Quirúrgica de `.agents/`

```bash
clean_agents() {
    local target_agents="$1/.agents"
    [ -d "$target_agents" ] || return 0
    for group in "$PAYLOAD/.agents"/*/; do
        local gname
        gname="$(basename "$group")"
        [ -d "$target_agents/$gname" ] || continue
        for item in "$group"*; do
            [ -e "$item" ] || continue
            rm -rf "$target_agents/$gname/$(basename "$item")"
        done
        rmdir "$target_agents/$gname" 2>/dev/null || true
    done
    rmdir "$target_agents" 2>/dev/null || true
}
```

#### 4. Purga y Limpieza de `docs/`

```bash
if [ "$PURGE" -eq 1 ]; then
    rm -rf "$TARGET/specs" "$TARGET/AGENTS.md" "$TARGET/MEMORY.md"
    rm -f "$TARGET/docs/constitution.md"
    rmdir "$TARGET/docs" 2>/dev/null || true
fi
```

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Implementación en Bash puro (`uninstall.sh`) en simetría con `install.sh`.**
  - _Alternativa descartada:_ Crear un subcomando `seb uninstall` en Go.
  - _Justificación:_ El motor Go compilado reside en `.seb/bin/seb`. Un proceso Go en ejecución que intente borrar su propio ejecutable y su directorio contenedor `.seb/` genera problemas de concurrencia y bloqueo de inodos en múltiples sistemas operativos. Además, si un proyecto no pudo compilar Go o se instaló con `--no-build`, la desinstalación debe poder ejecutarse sin depender del compilador Go.
- **Decisión 2: Suite de integración implementada en Go (`engine/cmd/seb/uninstall_test.go`).**
  - _Alternativa descartada:_ Script bash ad-hoc de pruebas sin integración con el arnés.
  - _Justificación:_ Respeta la Sección B de `docs/constitution.md`, permitiendo que `go test -v ./...` y `.seb/bin/seb verify desinstalacion_arnes` ejecuten automáticamente las pruebas deterministas con aislamiento en carpetas temporales (`t.TempDir()`).
- **Decisión 3: Verificación diferencial contra `payload/.agents/`.**
  - _Alternativa descartada:_ `rm -rf $TARGET/.agents`.
  - _Justificación:_ Los desarrolladores pueden crear skills o rules propios para su proyecto dentro de `.agents/`. Borrar toda la carpeta rompería componentes del usuario no relacionados con el arnés.

## 5. Secuencia de Tareas Atómicas (TDD)

- [ ] **T1. Estructura base de `uninstall.sh` y validación de argumentos/seguridad** (`RF-01`, `RF-09`, `RF-10`, `RF-11`, `RF-18`)
  - _Archivos:_ `uninstall.sh`, `engine/cmd/seb/uninstall_test.go`
  - _Hecho cuando:_ El comando `CHK-01` devuelve exit code `0`.
- [ ] **T2. Reversión de Git hooks y restauración de respaldos** (`RF-02`, `RF-03`)
  - _Archivos:_ `uninstall.sh`, `engine/cmd/seb/uninstall_test.go`
  - _Hecho cuando:_ El comando `CHK-02` devuelve exit code `0`.
- [ ] **T3. Limpieza quirúrgica de `.gitignore` y eliminación de `.seb/`** (`RF-04`, `RF-05`)
  - _Archivos:_ `uninstall.sh`, `engine/cmd/seb/uninstall_test.go`
  - _Hecho cuando:_ El comando `CHK-03` devuelve exit code `0`.
- [ ] **T4. Remoción quirúrgica de `.agents/` preservando componentes propios** (`RF-06`, `RF-07`)
  - _Archivos:_ `uninstall.sh`, `engine/cmd/seb/uninstall_test.go`
  - _Hecho cuando:_ El comando `CHK-04` devuelve exit code `0`.
- [ ] **T5. Reversión de instalación en modo dogfooding (`--self`)** (`RF-08`)
  - _Archivos:_ `uninstall.sh`, `engine/cmd/seb/uninstall_test.go`
  - _Hecho cuando:_ El comando `CHK-05` devuelve exit code `0`.
- [ ] **T6. Modo de simulación pasiva (`--dry-run`)** (`RF-12`)
  - _Archivos:_ `uninstall.sh`, `engine/cmd/seb/uninstall_test.go`
  - _Hecho cuando:_ El comando `CHK-06` devuelve exit code `0`.
- [ ] **T7. Modo de purga destructiva (`--purge`, `--force`), validación de TTY y limpieza de `docs/`** (`RF-13`, `RF-14`, `RF-15`, `RF-16`, `RF-17`)
  - _Archivos:_ `uninstall.sh`, `engine/cmd/seb/uninstall_test.go`
  - _Hecho cuando:_ El comando `CHK-07` devuelve exit code `0`.
- [ ] **T8. Idempotencia y validación de modo estricto Bash** (`RNF-01`, `RNF-02`)
  - _Archivos:_ `uninstall.sh`, `engine/cmd/seb/uninstall_test.go`
  - _Hecho cuando:_ El comando `CHK-08` devuelve exit code `0`.
- [ ] **T9. Verificación estática y suite global sin regresiones** (`RNF-Stack`, `GLOBAL`)
  - _Archivos:_ `engine/cmd/seb/uninstall_test.go`, `README.md`
  - _Hecho cuando:_ Los comandos `CHK-98` y `CHK-99` devuelven exit code `0`.
