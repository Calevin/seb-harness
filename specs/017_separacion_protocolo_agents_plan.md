# Plan Técnico — separacion_protocolo_agents (Spec 017)

- **Spec Origen:** `specs/017_separacion_protocolo_agents_spec.md`
- **Verificador Asociado:** `specs/017_separacion_protocolo_agents_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

El diseño separa de forma estricta la **propiedad del arnés** (`.agents/`) de la **propiedad del proyecto consumidor** (raíz del repositorio), habilitando carga progresiva de contexto (*progressive disclosure*):

1. **Protocolo SDD completo gestionado (`payload/.agents/SEB-HARNESS.md`):** Concentra todas las reglas operativas del flujo SDD (fuentes de verdad, fases 0–6, tabla de menor privilegio, herramientas `.seb/bin/seb`, regla de introspección, máquina de estados, identificadores prefijados `<NNN>-<TIPO>-<NN>` y operaciones one-off). Al vivir bajo `payload/.agents/`, hereda de forma nativa la sobrescritura total en `install.sh`, el enlace simbólico en modo `--self` y la remoción quirúrgica en `uninstall.sh`.
2. **Enrutador liviano con bloque gestionado (`payload/AGENTS.md` y `AGENTS.md` raíz):** Se reduce a un bloque compacto (máx. 30 líneas) delimitado por `<!-- BEGIN seb-harness:managed -->` y `<!-- END seb-harness:managed -->` que declara la firma `seb-harness`, la jerarquía base (`docs/constitution.md`, `MEMORY.md`) y la redirección a `.agents/SEB-HARNESS.md`, dejando libre el exterior para directivas propias del proyecto.
3. **Regla global mínima (`payload/.agents/rules/00-seb-harness.md`):** Se recorta (máx. 35 líneas) a los invariantes universales aplicables en todo turno (lectura de `AGENTS.md`/`constitution.md`/`MEMORY.md`, anti-reward-hacking, integridad de tests y sujeción a `seb guard` + `.agents/SEB-HARNESS.md`), eliminando la duplicación pesada del protocolo SDD.
4. **Motor determinista de bloques en shell POSIX (`install.sh` y `uninstall.sh`):** Se encapsulan funciones puras en `bash` + `awk` estándar (sin dependencias externas) para validar marcadores, sustituir atómicamente el bloque gestionado sin expansión de caracteres especiales (`$`, `&`, `\`, `/`, backticks), respaldar archivos legados a `AGENTS.legacy-seb.md` y remover el bloque en desinstalación estándar.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica | Requisitos Cubiertos (`017-RF-XX`) |
| :------------- | :----------------------------- | :---------------------- | :--------------------------------- |
| `payload/.agents/SEB-HARNESS.md` | `CREAR` | Protocolo operativo SDD completo gestionado por el arnés con aviso de sobrescritura. | `017-RF-01`, `017-RF-02`, `017-RF-13` |
| `payload/AGENTS.md` | `MODIFICAR` | Plantilla de enrutador liviano (<= 30 líneas en bloque gestionado) con marcadores `BEGIN`/`END`. | `017-RF-03`, `017-RNF-02` |
| `AGENTS.md` | `MODIFICAR` | Sincronización del `AGENTS.md` raíz del repositorio para paridad en dogfooding (`--self`). | `017-RF-11`, `017-RF-13` |
| `payload/.agents/rules/00-seb-harness.md` | `MODIFICAR` | Reducción de la regla global `always_on` a invariantes universales (<= 35 líneas). | `017-RF-08`, `017-RNF-02`, `017-RF-13` |
| `install.sh` | `MODIFICAR` | Validación estricta de marcadores, reemplazo atómico vía `awk`, migración de formato legado (`AGENTS.legacy-seb.md`), ejecución también en `--self` y retiro de `AGENTS.md.patch.md`. | `017-RF-02`, `017-RF-04`, `017-RF-05`, `017-RF-06`, `017-RF-07`, `017-RF-09`, `017-RF-11`, `017-RNF-01`, `017-RNF-03`, `017-CL-01`..`017-CL-04` |
| `uninstall.sh` | `MODIFICAR` | Remoción quirúrgica del bloque gestionado en `AGENTS.md` sin `--purge` (eliminando el archivo si queda vacío, respetando `--dry-run` y preservando ante marcadores malformados). | `017-RF-10`, `017-RNF-03`, `017-CL-01`, `017-CL-02`, `017-CL-05` |
| `engine/cmd/seb/managed_protocol_test.go` | `CREAR` | Suite de integración completa cubriendo protocolo gestionado, marcadores, migración legado, errores, `--self` y desinstalación. | `017-RF-01`..`017-RF-12`, `017-RNF-01`..`017-RNF-03`, `017-CL-01`..`017-CL-05` |
| `engine/cmd/seb/installer_documentador_test.go` | `MODIFICAR` | Actualización de expectativas obsoletas (`AGENTS.md.patch.md` -> respaldo legado + `.agents/SEB-HARNESS.md`) manteniendo >= conteo de aserciones. | `017-RF-13` |
| `engine/cmd/seb/skill_definition_test.go` | `MODIFICAR` | Reorientación de aserciones de Fase 6 y matriz de permisos hacia `.agents/SEB-HARNESS.md` y referencia en enrutador/regla. | `017-RF-13` |
| `engine/cmd/seb/prefixed_directives_test.go` | `MODIFICAR` | Reorientación de literales de identificadores prefijados y one-off hacia `.agents/SEB-HARNESS.md` preservando paridad de `AGENTS.md`. | `017-RF-13` |

## 3. Contratos, Firmas y Modelo de Datos

### Marcadores Canónicos
- Inicio: `<!-- BEGIN seb-harness:managed -->`
- Fin: `<!-- END seb-harness:managed -->`

### Contrato de Funciones Shell (`install.sh` y `uninstall.sh`)
```text
validate_managed_markers(file) -> exit code:
  0: exactamente 1 marcador BEGIN y 1 marcador END, con linea(BEGIN) < linea(END)
  2: 0 marcadores BEGIN y 0 marcadores END (archivo sin bloque gestionado)
  1: cualquier otro caso (huerfano, invertido o duplicado)

extract_managed_block(template_file, out_file):
  Extrae con awk desde BEGIN hasta END (ambos inclusive) sin interpretar secuencias.

replace_managed_block(target_file, template_file):
  1. Extrae bloque gestionado a archivo temporal.
  2. Con awk, emite lineas < begin_line tal cual, inserta el archivo temporal linea por linea via getline (literal, sin gsub/sub), y emite lineas > end_line tal cual.
  3. Reemplaza target_file de forma atomica (mv) preservando contenido usuario exacto.

sync_agents_md(target_file, template_file):
  - Si no existe target_file: cp template_file -> target_file.
  - Si existe: evaluar validate_managed_markers(target_file):
    - 0 -> replace_managed_block(target_file, template_file)
    - 1 -> imprimir diagnostico en stderr y abortar con exit 1 sin tocar target_file
    - 2 -> si has_seb_signature(target_file):
             respaldar a AGENTS.legacy-seb.md (sin sobrescribir si ya existe) + cp template_file -> target_file
           si no:
             backup_pre_seb(target_file) (AGENTS.pre-seb.md) + cp template_file -> target_file

strip_managed_block_on_uninstall(target_file, dry_run):
  - Evaluar validate_managed_markers(target_file):
    - 2 -> no modificar (conserva comportamiento historico para archivos sin bloque)
    - 1 -> advertir en stderr y preservar intacto
    - 0 -> remover lineas begin_line..end_line (inclusive) colapsando solo la linea en blanco inmediata si queda borde duplicado.
           Si el resultado no contiene caracteres no-blancos: eliminar target_file (o reportar en dry-run).
           Si queda contenido propio: escribir resultado atomicamente (o reportar en dry-run).
```

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Procesamiento de bloques con `awk` + `getline` en lugar de `sed` o `python3`.**
  - _Alternativa descartada:_ `sed` interpreta `&`, `\1` y delimitadores `/` en sustituciones (rompería `017-CL-01`); `python3` no está garantizado en todos los entornos mínimos POSIX (`017-RNF-03`). `awk` con lectura literal por archivo (`getline`) es POSIX estándar, inmune a metacaracteres y ya se usa en `install.sh`.
- **Decisión 2: Respaldo de formato legado como `AGENTS.legacy-seb.md` separado de `AGENTS.pre-seb.md`.**
  - _Alternativa descartada:_ Reutilizar `AGENTS.pre-seb.md` provocaría que `uninstall.sh` restaurase un archivo antiguo de `seb-harness` como si fuera un archivo previo ajeno al arnés (rompiendo el contrato de Spec 007).
- **Decisión 3: Mantener `GEMINI.md` y `CLAUDE.md` apuntando solo a `AGENTS.md` (`017-RF-12`).**
  - _Alternativa descartada:_ Referenciar `.agents/SEB-HARNESS.md` desde los stubs acoplaría los asistentes directamente al flujo SDD pesado y dificultaría el enrutamiento futuro de un flujo rápido.

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

_Regla para el Agente Implementador: Ejecuta UNA tarea a la vez en orden de dependencia. Primero escribe el test y verifica que falle (Rojo), luego escribe el código mínimo hasta pasar (Verde)._

- [ ] **T1. Crear protocolo gestionado `.agents/SEB-HARNESS.md`, enrutador `AGENTS.md` y regla global reducida `00-seb-harness.md`** (`017-RF-01`, `017-RF-02`, `017-RF-03`, `017-RF-08`, `017-RF-12`, `017-RNF-02`)
  - _Archivos:_ `engine/cmd/seb/managed_protocol_test.go`, `payload/.agents/SEB-HARNESS.md`, `payload/AGENTS.md`, `AGENTS.md`, `payload/.agents/rules/00-seb-harness.md`
  - _Hecho cuando:_ El comando `017-CHK-01` de `specs/017_separacion_protocolo_agents_verificador.md` devuelve exit code `0`.
- [ ] **T2. Implementar `sync_agents_md` con marcadores, reemplazo literal `awk` e idempotencia en `install.sh`** (`017-RF-04`, `017-RF-05`, `017-RNF-01`, `017-RNF-03`, `017-CL-01`, `017-CL-03`)
  - _Archivos:_ `engine/cmd/seb/managed_protocol_test.go`, `install.sh`
  - _Hecho cuando:_ El comando `017-CHK-02` devuelve exit code `0`.
- [ ] **T3. Implementar migración automática de formato legado (`AGENTS.legacy-seb.md`) y retirar generación de `AGENTS.md.patch.md` en `install.sh`** (`017-RF-06`, `017-RF-09`, `017-CL-04`)
  - _Archivos:_ `engine/cmd/seb/managed_protocol_test.go`, `install.sh`
  - _Hecho cuando:_ El comando `017-CHK-03` devuelve exit code `0`.
- [ ] **T4. Implementar validación estricta con aborto (`exit 1`) ante marcadores malformados en `install.sh`** (`017-RF-07`, `017-CL-02`)
  - _Archivos:_ `engine/cmd/seb/managed_protocol_test.go`, `install.sh`
  - _Hecho cuando:_ El comando `017-CHK-04` devuelve exit code `0`.
- [ ] **T5. Implementar remoción quirúrgica del bloque gestionado en `uninstall.sh` (incluyendo `--dry-run`, archivo vacío y marcadores malformados)** (`017-RF-10`, `017-CL-01`, `017-CL-02`, `017-CL-05`)
  - _Archivos:_ `engine/cmd/seb/managed_protocol_test.go`, `uninstall.sh`
  - _Hecho cuando:_ El comando `017-CHK-05` devuelve exit code `0`.
- [ ] **T6. Extender sincronización de `AGENTS.md` al modo `--self` en `install.sh`** (`017-RF-11`, `017-RNF-01`)
  - _Archivos:_ `engine/cmd/seb/managed_protocol_test.go`, `install.sh`
  - _Hecho cuando:_ El comando `017-CHK-06` devuelve exit code `0`.
- [ ] **T7. Actualizar tests de contratos reemplazados (`010`, `013`) y validar suite completa + `go vet`** (`017-RF-13`, `GLOBAL`)
  - _Archivos:_ `engine/cmd/seb/installer_documentador_test.go`, `engine/cmd/seb/skill_definition_test.go`, `engine/cmd/seb/prefixed_directives_test.go`
  - _Hecho cuando:_ Los comandos `017-CHK-07`, `017-CHK-98` y `017-CHK-99` devuelven exit code `0`.
