# MEMORY.md — Bitácora Técnica y Restricciones (seb-harness)

> **Regla de Higiene de Contexto:** Este archivo almacena lo aprendido empíricamente en el proyecto entre sesiones.
> - **Límite duro:** Máximo 150 líneas (~15 KB). Resume o elimina entradas obsoletas.
> - **Promoción:** Si un aprendizaje se vuelve una ley permanente de arquitectura, muévelo a `docs/constitution.md`.
> - **Seguridad:** Prohibido guardar credenciales, tokens, rutas privadas del host o datos personales.
> - **Invariante del Agente:** Antes de proponer un diseño en `seb-planificador` o escribir código, verifica las secciones 2 y 3 para no reintentar enfoques fallidos.

## 1. Estado Actual del Proyecto
- **Fase SDD activa:** Ciclo completado para Spec 010 (`010_integracion_documentador`); verificado y sellado con éxito.
- **Última Spec verificada:** 010 (`010_integracion_documentador`).
- **Estado de la suite de verificación:** Motor Go validado (100% de controles en `[PASA]`, `go vet` y 47 tests limpios).

## 2. Callejones sin Salida y Enfoques Descartados (NO REINTENTAR)
*Registro de librerías, patrones o configuraciones que se probaron en este proyecto y fallaron o resultaron incompatibles.*
- `[2026-10-22] [Inicialización] Guardar hashes y secuencia de specs en SQLite -> Falló en sincronización multi-máquina (notebook/PC vía git pull con DB local vacía) -> Estado y hashes sellados en archivos planos versionados (.seb/seb.lock, specs/) y SQLite solo para auditoría pasiva local.`

## 3. Restricciones Técnicas y Trampas Conocidas (Gotchas)
*Limitaciones del entorno (NixOS, kernel, contenedores, base de datos, versiones del compilador/runtime) o del dominio que no son obvias leyendo el código.*
- `[NixOS / Runtime] Binario go y sqlite no están en PATH global -> Invocaciones directas fallan con command not found -> Usar siempre 'direnv exec .' o 'nix develop --command'`
- `[Go Engine] Compilación sin CGO_ENABLED=0 genera binarios dependientes de glibc dinámica -> Compilar siempre con CGO_ENABLED=0 y flags -ldflags="-s -w"`

## 4. Riesgos Conocidos y Deuda Técnica Activa
*Riesgos arquitectónicos o casos borde postergados conscientemente para futuras specs.*
- *(Vacío inicialmente — Formato requerido: `[ID-Spec] Descripción del riesgo -> Condición de mitigación`)*

## 5. Decisiones Técnicas Relevantes (Histórico Reciente)
- `[D-01] Adopción de pure Go SQLite (modernc.org/sqlite) -> CGO SQLite (mattn/go-sqlite3) -> Eliminar necesidad de toolchain C en NixOS/cross-compilation.`
- `[D-02] Telemetría dual e híbrida -> Captura solo vía bash hooks -> Centralización de cascada de tarea activa en Go, cronometraje de comandos internos, soporte de archivos usage.jsonl y cálculo de costos con .seb/config.json.`
- `[D-03] Homogeneización de Nomenclatura SDD -> Estandarización de prefijo secuencial <NNN>_ en spec, plan y verificador. Clave canónica <NNN>_<tarea> en seb.lock y resolución flexible (ID, nombre simple, canónico) en CLI con rechazo de ambigüedad.`
- `[D-04] Gestión de Grafo de Dependencias (DAG) -> Persistencia en seb.lock con verified: true y commit_hash -> Guardarraíl determinista en plan-lock que valida prerrequisitos directos y previene referencias circulares mutuas de forma offline sin depender de SQLite local.`
- `[D-05] Parser Híbrido de Matrices de Verificación y Anti-Silent-Drop -> Sustitución de regex rígida en parseVerifierFile por analizador contextual (inmunidad en bloques cercados) y extracción delimitada por backticks externos -> Soporte de requisitos heterogéneos (RF, RNF, GLOBAL, CL) con normalización de espacios, aislamiento de comandos de shell con pipes/operadores y rechazo bloqueante inmediato (código 1) ante cualquier fila CHK- malformada o ID duplicado, erradicando omisiones silenciosas y falsos aprobados.`
- `[D-06] Asimilación de Contexto Brownfield y Reversibilidad -> Respaldo automático determinista con infijo .pre-seb.<ext> en install.sh ante archivos ajenos sin firma seb-harness (AGENTS.md, GEMINI.md, CLAUDE.md) -> Asimilación en docs/constitution.md y generación de stubs anti-split-brain en seb-iniciador -> Restauración quirúrgica de archivos *.pre-seb.* en uninstall.sh garantizando reversibilidad total.`
- `[D-07] Estado Determinista de Sesión y Aislamiento de Habilidades -> Separación de identidad de gobernanza SDD frente a lecturas pasivas de transcripción -> Gestión de rol mediante CLI (seb role set/get/clear) persistido en archivo local .seb/ACTIVE_ROLE -> Precedencia estricta de 4 niveles (SEB_SKILL > ACTIVE_ROLE > transcripción seb-* > implementador) y herencia segura de permisos de implementador en checkPermission para habilidades de dominio externas (como Spring Boot o React), erradicando falsos bloqueos en proyectos brownfield.`
- `[D-08] Cancelación Formal y Purga Atómica de Tareas -> Soporte dual 'seb abort <tarea> [--purge] [--force|-f]' y alias 'seb task abort' -> Aborto formal por defecto retiene telemetría/costos en SQLite, actualiza spec en disco y sella 'aborted: true' en seb.lock invalidando dependencias en el DAG -> Purga atómica con --purge elimina físicamente artefactos en specs/, suprime registros transaccionalmente en SQLite y libera el identificador secuencial para spec-next.`
- **[2026-11-10] [Decisión Técnica]** Spec 010: Integración de Fase 6 (seb-documentador) con guardarraíl de menor privilegio restringido a docs/use-cases/ y generación asistida de AGENTS.md.patch.md en install.sh.
- **[2026-11-18] [Desbloqueo]** Spec 013 (identificadores_unicos_requisitos) desbloqueada tras verificarse la Spec 014 (Afecta-UC multiple); re-especificada con migracion homogenea de identificadores sin punto de corte.
- **[2026-11-18] [Deuda Técnica / Riesgo]** Spec 014: CHK-06 fija literalmente los SHA-256 de spec y verificador 011; un resellado intencional de la 011 hara fallar la verificacion de la 014.
- **[2026-11-18] [Decisión Técnica]** Spec 013: Identificadores <NNN>-<TIPO>-<sufijo> como unico formato valido (sin punto de corte). Homogeneizacion via operacion one-off 013 ejecutada solo con rol 'migrador' (nuevo mecanismo de one-offs identificados por el ID de la spec que los define). Unica excepcion autorizada a la inmutabilidad de specs/verificadores sellados; pendiente promoverla a docs/constitution.md via seb-iniciador.
- **[2026-11-18] [Riesgo / Secuencia de Cierre]** Spec 013: el verificador 013 se sella en formato heredado (CHK-NN) porque el binario vigente no reconoce 013-CHK-*. Tras compilar el motor nuevo, seb verify y el hook pre-commit fallan sobre verificadores heredados hasta que el humano ejecute la one-off 013 con rol migrador; no commitear codigo de la 013 antes de migrar o usar el orden del DoD.
- **[2026-11-18] [Migración Pendiente]** Spec 013: proyectos consumidores de seb-harness quedaran bloqueados en seb verify tras actualizar hasta ejecutar la one-off 013 (rol migrador). Referencias ambiguas reportadas por la migracion requieren revision manual.
- **[2026-11-18] [Deuda Técnica / Riesgo]** Spec 013: 013-CHK-07, 013-CHK-11 y 013-CHK-97 dependen del commit f3f5945 en el historial Git (clones superficiales fallan) y 013-CHK-07 compara los verificadores historicos contra su migracion determinista: una futura one-off que los reescriba exigira re-especificar ese control.
- **[2026-11-18] [Riesgo / Migración]** Spec 013: las referencias ambiguas que la one-off 013 deja en specs y planes sellados NO deben editarse a mano (rompen el sello); solo pueden corregirse en MEMORY.md y docs/use-cases. Re-verificacion historica 001-012 y 014 restampo commit_hash al HEAD vigente.
- **[2026-11-25] [Deuda Técnica / Riesgo]** Spec 017 (017-RF-13): Los controles historicos 016-CHK-15 y 016-CHK-16 buscan literales en payload/AGENTS.md que ahora residen en payload/.agents/SEB-HARNESS.md; una re-verificacion historica de la 016 fallara en esos dos checks salvo one-off.
