# MEMORY.md — Bitácora Técnica y Restricciones (seb-harness)

> **Regla de Higiene de Contexto:** Este archivo almacena lo aprendido empíricamente en el proyecto entre sesiones.
> - **Límite duro:** Máximo 150 líneas (~15 KB). Resume o elimina entradas obsoletas.
> - **Promoción:** Si un aprendizaje se vuelve una ley permanente de arquitectura, muévelo a `docs/constitution.md`.
> - **Seguridad:** Prohibido guardar credenciales, tokens, rutas privadas del host o datos personales.
> - **Invariante del Agente:** Antes de proponer un diseño en `seb-planificador` o escribir código, verifica las secciones 2 y 3 para no reintentar enfoques fallidos.

## 1. Estado Actual del Proyecto
- **Fase SDD activa:** Constitución completada (`docs/constitution.md` generada y activa).
- **Última Spec verificada:** Ninguna.
- **Estado de la suite de verificación:** Pendiente de primera spec. Motor Go validado (`go vet` y `go test` limpios).

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
