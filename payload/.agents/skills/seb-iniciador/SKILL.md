---
name: seb-iniciador
description: "Paso 0 de seb-harness. Analiza un proyecto existente (brownfield) o nuevo y genera docs/constitution.md con las secciones A-F, inicializando además MEMORY.md y la base de datos de auditoría. Úsalo una vez por proyecto."
---

# Skill: seb-iniciador (Generador de Constitución SDD)

## Context & Role

You are operating as the **`seb-iniciador`** skill within **`seb-harness`**.
Your sole objective is to reverse-engineer an existing repository (or bootstrap a new one) to generate **`docs/constitution.md`**—the immutable architectural and domain law of the project—and ensure `MEMORY.md` and `.seb/seb-audit.db` are initialized.

## Least Privilege Constraints (Strictly Enforced by `.seb/bin/seb guard`)

- **Active Role:** `seb-iniciador` (export `SEB_SKILL="seb-iniciador"` when invoking shell commands).
- **Allowed Reads:** Project manifests, configuration files, directory trees, READMEs, CI workflows, and sample domain/test files.
- **Allowed Writes:** ONLY `docs/constitution.md`, `docs/constitution.pre-seb.md`, `MEMORY.md`, `AGENTS.md`, and assistant stubs (`GEMINI.md`, `CLAUDE.md`).
- **Forbidden:** You MUST NOT modify any source code, tests, dependencies, or files inside `specs/`.

## Step-by-Step Execution Protocol

### Phase 1: Harness Initialization & Surgical Reconnaissance (Low Token Footprint)

1. If `.seb/seb-audit.db` does not exist and `.seb/bin/seb` is compiled, run:
   `SEB_SKILL="seb-iniciador" .seb/bin/seb init`
2. Do NOT read the entire codebase. Inspect ONLY high-signal files to infer the stack, architecture, and verification commands:
   - **Nix / OS Env:** `flake.nix`, `shell.nix`, `.envrc`, `Makefile`, `justfile`, `Dockerfile`, `docker-compose.yml`.
   - **Go:** `go.mod`, `go.sum`, `.golangci.yml`, main entrypoints (`cmd/`), and 1-2 `*_test.go` files.
   - **Java / JVM:** `pom.xml`, `build.gradle`, `build.gradle.kts`, `src/main/`, `src/test/`.
   - **PHP / Symfony:** `composer.json`, `symfony.lock`, `phpunit.xml.dist`, `phpstan.neon`, `config/`, `src/`.
   - **JS / TS / Python / Others:** `package.json`, `pyproject.toml`, etc.
   - **CI & Linters:** `.github/workflows/*`, `.gitlab-ci.yml`, `.editorconfig`.
   - **Existing Context & Pre-seb Backups:** `README.md`, `AGENTS.md`, `AGENTS.pre-seb.md`, `GEMINI.md`, `GEMINI.pre-seb.md`, `CLAUDE.md`, `CLAUDE.pre-seb.md`, `docs/constitution.md`, `docs/constitution.pre-seb.md`, `MEMORY.md`, `TODO.md`.

### Phase 2: Brownfield Constitution Protocol & Gap Detection

1. **Preexisting Constitution Check:**
   - If `docs/constitution.md` already exists and contains the canonical `seb-harness` signature (case-insensitive): **PRESERVE IT**. Do not overwrite or destructively modify it.
   - If `docs/constitution.md` exists but lacks the `seb-harness` signature: backup the original as `docs/constitution.pre-seb.md` and assimilate its domain/stack rules into the new constitution.
2. Compare your findings (including rules extracted from `*.pre-seb.md` and preexisting instructions) against required sections **A through F**.
3. If critical decisions cannot be inferred from the code or legacy context, ask the user **up to 4 direct, numbered questions** before writing the file.
4. If the codebase and legacy context provide sufficient directives, present a concise draft summary and write `docs/constitution.md`. Consolidate legacy architecture, domain and style rules into sections B, C, D and E.

### Phase 3: Generate `docs/constitution.md` (Mandatory Structure A–F)

Write `docs/constitution.md` in **Spanish** (keeping technical terms, CLI commands, and code identifiers in their original language). Keep it dense, verifiable, and free of fluff. Every rule must be actionable and verifiable.

Use this exact structure:

```markdown
# Constitución del Proyecto — [Nombre del Proyecto]

> **Ley Suprema (`seb-harness`):** Este documento define los principios innegociables del proyecto. Toda especificación (`*_spec.md`), plan (`*_plan.md`), verificador (`*_verificador.md`) e implementación debe cumplirlos estrictamente.

## A. Naturaleza del Proyecto

- **Propósito y Misión:** [Qué problema resuelve el software y cuál es su valor principal]
- **Usuarios / Consumidores:** [Quién interactúa con el sistema: usuarios finales, otros microservicios, CLI, APIs]
- **Tipo de Sistema y Alcance:** [Monolito modular, microservicio, CLI, librería, etc. y límites claros del sistema]

## B. Stack Tecnológico (Reglas de Implementación)

- **Lenguaje y Runtime:** [Ej. Go 1.23+, Java 21, PHP 8.3 + Symfony X, entorno NixOS]
- **Frameworks y Librerías Core:** [Dependencias oficiales ya presentes en el proyecto que DEBEN reutilizarse]
- **Persistencia e Infraestructura:** [Motores de base de datos, colas, caché, contenedores]
- **Comandos Oficiales de Verificación Programática:**
  - **Compilación / Build:** `[comando exacto]`
  - **Tests Unitarios / Integración:** `[comando exacto, ej. go test ./..., mvn test, ./vendor/bin/phpunit]`
  - **Linter / Análisis Estático:** `[comando exacto, ej. golangci-lint run, ./vendor/bin/phpstan]`
- **Prohibiciones de Stack:**
  - Prohibido añadir nuevas dependencias externas sin aprobación explícita en la spec/plan.
  - Prohibido reemplazar librerías existentes en el proyecto por alternativas equivalentes.

## C. Reglas de Dominio y Lógica de Negocio

- **Invariantes del Dominio:** [Reglas de negocio sagradas que nunca pueden violarse]
- **Fronteras de Datos y Consistencia:** [Reglas sobre transacciones, idempotencia, zonas horarias, precisión monetaria/numérica, compatibilidad hacia atrás]
- **Aislamiento de Lógica:** [La lógica de negocio debe ser pura o desacoplada de la capa de transporte/UI para permitir testeo determinista sin levantar infraestructura innecesaria]

## D. Estructura y Estilo de Código

- **Topología de Directorios:** [Mapa breve de las carpetas clave del repositorio y la responsabilidad de cada una]
- **Patrones Arquitectónicos:** [Hexagonal, Clean Architecture, MVC, DDD, separación de capas]
- **Convenciones de Nomenclatura e Idioma:** [Idioma del código, comentarios, commits y documentación]
- **Prohibiciones de Diseño:** [Antipatrones prohibidos en este repo: ej. lógica en controladores, estado global mutable, acoplamiento circular]

## E. Manejo de Errores y Validaciones

- **Estrategia de Errores:** [Excepciones tipadas vs retorno explícito de errores, códigos HTTP/gRPC estándar, wrapping de contexto]
- **Validación de Entradas:** [Dónde y cómo se validan los DTOs/requests antes de tocar el dominio]
- **Observabilidad y Seguridad:** [Formato de logs estructurados, prohibición absoluta de loggear secretos, credenciales o PII]

## F. Comportamiento del Agente de IA (Reglas SDD — `seb-harness`)

1. **La Spec Manda:** Ninguna funcionalidad se implementa sin un archivo `specs/<NNN>_<tarea>_spec.md` aprobado y sin etiquetas `[NECESITA ACLARACIÓN]`.
2. **Testabilidad Programática Obligatoria:** Ningún plan (`specs/<NNN>_<tarea>_plan.md`) es válido sin su correspondiente `specs/<NNN>_<tarea>_verificador.md` con comandos que devuelvan código de salida `0`.
3. **Anti-Reward-Hacking:** Está terminantemente prohibido modificar `docs/constitution.md`, `specs/*_spec.md` o `specs/*_verificador.md` durante la implementación, así como deshabilitar o relajar tests para forzar que pasen.
4. **Memoria Persistente (`MEMORY.md`):** Antes de planificar o codificar, el agente debe consultar `MEMORY.md` para evitar callejones sin salida ya documentados, y registrar allí cualquier nueva limitación técnica descubierta.
5. **Commits Protegidos:** Todo commit de código debe pasar la puerta determinista `.seb/bin/seb verify <tarea>`.
```

### Phase 4: Anti-Split-Brain Assistant Stubs Generation

If external assistant context files were detected or backed up (`GEMINI.pre-seb.md`, `CLAUDE.pre-seb.md`, etc.), generate or overwrite the canonical assistant stubs (`GEMINI.md`, `CLAUDE.md`) pointing to the single source of truth:

```markdown
# Contexto del Proyecto

> Este proyecto implementa gobernanza y desarrollo determinista bajo **seb-harness**.
> Todas las directrices de arquitectura, stack y estilo están consolidadas en la constitución del proyecto.

- **Ley Suprema de Arquitectura y Dominio:** [docs/constitution.md](docs/constitution.md)
- **Protocolo Operativo de Agentes:** [AGENTS.md](AGENTS.md)
- **Bitácora y Memoria Técnica:** [MEMORY.md](MEMORY.md)
```
