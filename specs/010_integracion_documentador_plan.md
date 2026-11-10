# Plan Técnico — Integración de Skill Documentador Funcional (Spec 010)

- **Spec Origen:** `specs/010_integracion_documentador_spec.md`
- **Verificador Asociado:** `specs/010_integracion_documentador_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

La integración del skill `seb-documentador` como la Fase 6 del arnés abarca tres componentes arquitectónicos desacoplados:

1. **Gobernanza y Menor Privilegio en el Motor Go (`engine/cmd/seb/main.go`):**
   - Incorporación de `seb-documentador` al mapa de roles canónicos (`canonicalRoles`).
   - Normalización y validación estricta en `setActiveRole` y `cmdRole` para admitir `seb-documentador`.
   - Modificación de la función de guardarraíl `checkPermission`: bajo el rol `seb-documentador`, se autoriza la escritura exclusivamente en rutas con prefijo `docs/use-cases/`, en el archivo de índice `docs/use-cases/README.md` y en la memoria técnica `MEMORY.md`. Cualquier intento de modificar código fuente, tests, constitución (`docs/constitution.md`) o especificaciones/planes/verificadores (`specs/*`) es bloqueado de inmediato con salida no nula.
   - Creación inicial de los directorios `.agents/skills/seb-documentador` y `docs/use-cases` en `ensureDirectories`.

2. **Definición de Skill y Protocolo Operativo (`payload/` y `.agents/`):**
   - Creación del archivo de habilidad `payload/.agents/skills/seb-documentador/SKILL.md` (y su contraparte dogfooding en `.agents/skills/seb-documentador/SKILL.md`) detallando la estructura obligatoria de casos de uso (Ficha de intención, SSD en Mermaid con `autonumber`, árbol de decisiones, catálogo de errores y postcondiciones), prohibición absoluta de emojis y reglas de idioma (inglés para identificadores, español para prosa explicativa).
   - Actualización de las directivas maestras en `payload/AGENTS.md`, `AGENTS.md` y `payload/.agents/rules/00-seb-harness.md` incorporando la Fase 6 en la máquina de estados SDD y en la tabla de menor privilegio.

3. **Mecanismo de Aprovisionamiento y Parche Asistido (`install.sh` y `uninstall.sh`):**
   - Actualización de `install.sh` para garantizar la creación del directorio `docs/use-cases/` y la copia del nuevo skill.
   - Detección en `install.sh` de proyectos existentes donde `AGENTS.md` posea la firma de `seb-harness` pero carezca de la referencia a `seb-documentador`. En tal caso, genera de forma no destructiva el archivo auxiliar `AGENTS.md.patch.md` con el bloque Markdown sugerido y emite una advertencia en consola.
   - Confirmación en `uninstall.sh` de que los artefactos generados en `docs/use-cases/` permanecen intactos durante una desinstalación estándar.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica | Requisitos Cubiertos (`RF-XX`) |
| :--- | :--- | :--- | :--- |
| `engine/cmd/seb/main.go` | `MODIFICAR` | Agregar `seb-documentador` a roles canónicos, soporte en `ensureDirectories` y regla de menor privilegio en `checkPermission` | `RF-01`, `RF-02`, `RF-03`, `RNF-01`, `CL-01` |
| `engine/cmd/seb/documentador_guard_test.go` | `CREAR` | Pruebas unitarias de guardarraíl de permisos y gestión de roles para `seb-documentador` | `RF-02`, `RF-03`, `RNF-01` |
| `payload/.agents/skills/seb-documentador/SKILL.md` | `CREAR` | Especificación de la habilidad, heurísticas Mermaid, plantilla de caso de uso y reglas de idioma | `RF-04`, `RF-05`, `RF-06`, `RF-07`, `RF-08`, `RF-13`, `RNF-02`, `CL-02`, `CL-04` |
| `.agents/skills/seb-documentador/SKILL.md` | `CREAR` | Enlace o copia de dogfooding del skill documentador en el propio arnés | `RF-04`, `RF-05`, `RF-06` |
| `payload/AGENTS.md` | `MODIFICAR` | Incorporar la Fase 6 y la fila de `seb-documentador` en la tabla de menor privilegio de la plantilla | `RF-01`, `RF-02`, `RF-03` |
| `AGENTS.md` | `MODIFICAR` | Actualizar el protocolo operativo del arnés raíz con la Fase 6 | `RF-01`, `RF-02`, `RF-03` |
| `payload/.agents/rules/00-seb-harness.md` | `MODIFICAR` | Documentar la Fase 6 y guardarraíles deterministas en las reglas globales distribuidas | `RF-01`, `RF-02`, `RF-03` |
| `install.sh` | `MODIFICAR` | Creación de `docs/use-cases/`, copia del skill y generación asistida de `AGENTS.md.patch.md` en actualizaciones | `RF-09`, `RF-10`, `RF-11`, `RNF-03`, `CL-01`, `CL-03` |
| `uninstall.sh` | `MODIFICAR` | Asegurar la preservación de `docs/use-cases/` ante desinstalaciones estándar | `RF-12` |
| `engine/cmd/seb/installer_documentador_test.go` | `CREAR` | Pruebas automatizadas de aprovisionamiento, actualización idempotente y generación de parches | `RF-09`, `RF-10`, `RF-11`, `RF-12`, `RNF-03`, `CL-03` |
| `engine/cmd/seb/skill_definition_test.go` | `CREAR` | Pruebas de verificación de estructura, ausencia de emojis y reglas de idioma del skill | `RF-04`, `RF-05`, `RF-06`, `RF-07`, `RF-08`, `RF-13` |

## 3. Contratos, Firmas y Modelo de Datos

### Regla de Control de Acceso en `checkPermission` (`engine/cmd/seb/main.go`)

```go
case "seb-documentador":
    // Autorizado para escribir unicamente en el directorio de casos de uso y en la bitacora tecnica
    return strings.HasPrefix(target, "docs/use-cases/") ||
        target == "docs/use-cases/README.md" ||
        target == "MEMORY.md"
```

### Roles Canónicos en `main.go`

```go
var canonicalRoles = map[string]bool{
    "seb-iniciador":     true,
    "seb-especificador": true,
    "seb-clarificador":  true,
    "seb-planificador":  true,
    "seb-verificador":   true,
    "seb-documentador":  true, // Incorporacion oficial de la Fase 6
    "implementador":     true,
}
```

### Contrato del Parche Auxiliar en `install.sh`

```bash
# Si AGENTS.md ya existe y no menciona seb-documentador, se genera el parche
if [ -e "$TARGET/AGENTS.md" ] && has_seb_signature "$TARGET/AGENTS.md"; then
    if ! grep -q "seb-documentador" "$TARGET/AGENTS.md"; then
        generate_agents_patch "$TARGET/AGENTS.md.patch.md"
        warn "AGENTS.md no incluye la Fase 6. Se genero $TARGET/AGENTS.md.patch.md para facilitar su actualizacion."
    fi
fi
```

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Ausencia de subcomandos CLI adicionales (`seb doc`):**
  - *Justificación:* Mantiene el motor Go agnóstico y enfocado exclusivamente en guardarraíles, candados y telemetría, delegando el razonamiento y generación de diagramas al modelo de IA mediante el skill y sus directivas operativas.
- **Decisión 2: Preservación estricta de `AGENTS.md` y generación de `.patch.md`:**
  - *Justificación:* Evita que un script de actualización sobrescriba directivas personalizadas que el usuario humano haya configurado en su proyecto, otorgándole control total mediante un parche listo para aplicar.
- **Decisión 3: Estructura fija `docs/use-cases/UC-<dominio>_<accion>.md` y `docs/use-cases/README.md`:**
  - *Justificación:* Homogeneiza la organización de casos de uso entre proyectos, facilitando que el guardarraíl de menor privilegio aplique una comprobación de prefijo en O(1) con latencia `<1ms`.

## 5. Secuencia de Tareas Atómicas (TDD)

- [x] **T1. Guardarraíl de menor privilegio y registro de roles para `seb-documentador` (`TestDocumentadorGuardAndRoles`)** (`RF-01`, `RF-02`, `RF-03`, `RNF-01`, `CL-01`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/documentador_guard_test.go`
  - _Hecho cuando:_ El comando `CHK-01` de `specs/010_integracion_documentador_verificador.md` devuelve exit code `0`.
- [x] **T2. Aprovisionamiento, actualización idempotente y parche asistido en scripts (`TestInstallDocumentadorSkillAndPatch`)** (`RF-09`, `RF-10`, `RF-11`, `RF-12`, `RNF-03`, `CL-01`, `CL-03`)
  - _Archivos:_ `install.sh`, `uninstall.sh`, `engine/cmd/seb/installer_documentador_test.go`
  - _Hecho cuando:_ El comando `CHK-02` de `specs/010_integracion_documentador_verificador.md` devuelve exit code `0`.
- [x] **T3. Creación de la definición del skill `seb-documentador` y reglas operativas (`TestDocumentadorSkillDefinitionAndRules`)** (`RF-04`, `RF-05`, `RF-06`, `RF-07`, `RF-08`, `RF-13`, `RNF-02`, `CL-02`, `CL-04`)
  - _Archivos:_ `payload/.agents/skills/seb-documentador/SKILL.md`, `.agents/skills/seb-documentador/SKILL.md`, `payload/AGENTS.md`, `AGENTS.md`, `payload/.agents/rules/00-seb-harness.md`, `engine/cmd/seb/skill_definition_test.go`
  - _Hecho cuando:_ El comando `CHK-03` de `specs/010_integracion_documentador_verificador.md` devuelve exit code `0`.
- [x] **T4. Verificación global del arnés (linter oficial y suite completa sin regresiones)** (`RNF-01`, `RNF-03`, `GLOBAL`)
  - _Archivos:_ `engine/...`
  - _Hecho cuando:_ Los comandos `CHK-98` y `CHK-99` de `specs/010_integracion_documentador_verificador.md` devuelven exit code `0`.
