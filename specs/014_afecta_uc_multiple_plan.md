# Plan Técnico — Soporte de Múltiples Casos de Uso en el Metadato `Afecta-UC:` (Spec 014)

- **Spec Origen:** `specs/014_afecta_uc_multiple_spec.md`
- **Verificador Asociado:** `specs/014_afecta_uc_multiple_verificador.md`
- **Estado:** `PLANIFICADO`
- **Dependencias verificadas:** `010_integracion_documentador` (`fc38a2e`), `011_gobernanza_afecta_uc` (`851cff2`)

## 1. Resumen Arquitectónico y Aislamiento de Lógica

La tarea es puramente declarativa: modifica directivas y plantillas de `payload/.agents/skills/` y la suite de tests Go que las valida. No se toca la lógica productiva del motor (`engine/cmd/seb/main.go`) ni los archivos `docs/use-cases/*` (la migración real del catálogo a la columna "Specs de Evolución" la ejecutará `seb-documentador` en la Fase 6 de esta misma tarea, en cumplimiento de 014-RF-12).

Frentes de cambio:

1. **`seb-especificador`** (directiva y plantilla): sintaxis de lista, reglas de `nuevo` único y `ninguno` exclusivo, y reemplazo de la regla de división obligatoria por el criterio cohesivo/independiente.
2. **`seb-clarificador`**: auditoría de listas (sintaxis, existencia por elemento, orden irrelevante, respaldo por `RF-XX`, cobertura inversa) y extensión del Hard Gate.
3. **`seb-documentador`**: procesamiento de listas, resolución de `nuevo` en uno o más UC, columna "Specs de Evolución" con migración y no duplicación.
4. **Tests**: nueva suite `afecta_uc_multiple_test.go` y adaptación acotada (014-RF-14) de `afecta_uc_governance_test.go`.

Como `.agents` y `.claude/skills` son symlinks a `payload/.agents` en este repositorio, la sincronización local es estructural; la distribución a proyectos destino se valida ejecutando `install.sh --no-build` sobre un directorio temporal.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción | Responsabilidad Técnica | Requisitos Cubiertos |
| :--- | :--- | :--- | :--- |
| `payload/.agents/skills/seb-especificador/SKILL.md` | `MODIFICAR` | Sintaxis de lista, reglas de elementos, criterio multi-UC cohesivo/independiente, eliminación de la división obligatoria | `014-RF-01`, `014-RF-02`, `014-RF-04`, `014-RF-13`, `014-RF-16`, `014-CL-01` |
| `payload/.agents/skills/seb-especificador/template/nnn_nombre_tarea_spec.md` | `MODIFICAR` | Nueva línea de cabecera `Afecta-UC:` con sintaxis de lista | `014-RF-03` |
| `payload/.agents/skills/seb-clarificador/SKILL.md` | `MODIFICAR` | Auditoría de listas y extensión del Hard Gate de Fase 5 | `014-RF-05`, `014-RF-06`, `014-RF-07`, `014-CL-02`..`014-CL-06` |
| `payload/.agents/skills/seb-documentador/SKILL.md` | `MODIFICAR` | Procesamiento de listas, resolución de `nuevo`, columna "Specs de Evolución" | `014-RF-08`..`014-RF-12`, `014-CL-07` |
| `engine/cmd/seb/afecta_uc_multiple_test.go` | `CREAR` | Verificación de literales canónicos, ausencia de la regla sustituida, distribución vía `install.sh`, integridad de sellos 011 y specs previas | `RF-01`..`RF-17`, `RNF-01` |
| `engine/cmd/seb/afecta_uc_governance_test.go` | `MODIFICAR` (acotado, 014-RF-14) | Sustituir únicamente las dos aserciones de literales reemplazados en `TestEspecificadorAfectaUCDirectives` | `014-RF-14`, `014-RF-15` |

## 3. Contratos: Literales Canónicos

Los tests verifican la presencia literal (`strings.Contains`) de los siguientes textos. El implementador puede redactar libremente el resto del párrafo, pero estos literales deben aparecer exactamente así.

### 3.1 Plantilla (`template/nnn_nombre_tarea_spec.md`)

Línea de cabecera (reemplaza a la anterior):

```markdown
- **Afecta-UC:** [`UC-<dominio>_<accion>` | `nuevo` | `ninguno` | lista: `UC-<dominio>_<accion>`, `nuevo`]
```

### 3.2 `seb-especificador/SKILL.md`

| Clave | Literal requerido | Requisito |
| :--- | :--- | :--- |
| E1 | `` comma-and-space separated list `` | 014-RF-01, 014-RF-02 |
| E2 | `` each element individually wrapped in backticks `` | 014-RF-02 |
| E3 | `` `nuevo` may appear at most once `` | 014-RF-02 |
| E4 | `` `ninguno` is exclusive `` | 014-RF-02 |
| E5 | `` `nuevo` may be combined with existing `UC-<dominio>_<accion>` identifiers `` | 014-RF-02 |
| E6 | `` cohesive `` | 014-RF-04 |
| E7 | `` functionally independent `` | 014-RF-04 |
| E8 | `` A single-element list is equivalent to a single value `` | 014-CL-01 |

Literal **prohibido** (014-RF-13): `` to preserve a single unambiguous `Afecta-UC:` value ``.

### 3.3 `seb-clarificador/SKILL.md`

| Clave | Literal requerido | Requisito |
| :--- | :--- | :--- |
| C1 | `` When `Afecta-UC:` declares a list `` | 014-RF-05 |
| C2 | `` each `UC-<dominio>_<accion>` element exists in `docs/use-cases/README.md` `` | 014-RF-05, 014-CL-05 |
| C3 | `` The order of the elements is irrelevant `` | 014-RF-05, 014-CL-06 |
| C4 | `` backed by at least one `RF-XX` `` | 014-RF-06 |
| C5 | `` no `RF-XX` modifies a cataloged use case absent from the list `` | 014-RF-06 |
| C6 | `` `ninguno` combined with other elements `` | 014-RF-07, 014-CL-02 |
| C7 | `` duplicate elements (including repeated `nuevo`) `` | 014-RF-07, 014-CL-03, 014-CL-04 |
| C8 | `` a separator other than comma and space `` | 014-RF-07 |
| C9 | `` an invalid `Afecta-UC:` list `` (dentro de la sección del Hard Gate, posterior a `STRICTLY FORBIDDEN from marking a spec as `APROBADA``) | 014-RF-07 |

### 3.4 `seb-documentador/SKILL.md`

| Clave | Literal requerido | Requisito |
| :--- | :--- | :--- |
| D1 | `` Si `Afecta-UC:` declara una lista `` | 014-RF-08 |
| D2 | `` actualizar cada caso de uso `UC-<dominio>_<accion>` declarado `` | 014-RF-08 |
| D3 | `` determinar a partir de los `RF-XX` la cantidad de casos de uso nuevos `` | 014-RF-09 |
| D4 | `` identificador `UC-<dominio>_<accion>` inexistente en el catalogo `` | 014-RF-09 |
| D5 | `` columna "Specs de Evolución" inmediatamente despues de "Spec Originaria" `` | 014-RF-10 |
| D6 | `` sin modificar su "Spec Originaria" `` | 014-RF-11 |
| D7 | `` completar con `-` las filas `` | 014-RF-12 |
| D8 | `` sin duplicar rutas ya registradas `` | 014-CL-07 |

Además, el bloque de la plantilla de tabla del catálogo (si el skill la describe) debe incluir el encabezado `| Spec Originaria | Specs de Evolución |`.

## 4. Decisiones Técnicas Justificadas

- **D1: Literales canónicos en el plan en vez de redacción libre.** Las directivas son prosa para LLM; la única verificación determinista viable es por presencia de literales. Fijarlos en el plan evita que el implementador elija los tokens que su propio test comprueba (separación generador/evaluador). Limitación aceptada (ya presente en la Spec 011): se verifica que la regla esté escrita, no que un agente la aplique correctamente.
- **D2: No anidar `seb verify gobernanza_afecta_uc` dentro de `seb verify afecta_uc_multiple`.** Ejecutar un `verify` dentro de otro escribe en `seb.lock` y SQLite durante la verificación en curso y recompila el binario en ejecución. RF-15 se cubre de forma equivalente y sin efectos colaterales: (a) se ejecutan las tres funciones de test que componen `CHK-01..03` de la 011, (b) `CHK-98/99` de la 011 coinciden con los de esta spec, y (c) se compara el SHA-256 en disco de spec y verificador 011 contra los hashes literales sellados.
- **D3: Guardarraíl anti-relajación sobre el test adaptado (014-RF-14) en el propio verificador.** El verificador (inmutable para el implementador) comprueba con `grep` que `afecta_uc_governance_test.go` conserva sus aserciones críticas no sustituidas, no contiene `t.Skip` y mantiene al menos 17 llamadas `t.Errorf`/`t.Fatalf` (conteo actual).
- **D4: Distribución verificada vía `install.sh --no-build`.** Reutiliza el patrón de `TestInstallClaudeIntegration` (Spec 012) para cubrir RNF-01 en proyectos destino, incluido `.claude/skills`.

## 5. Secuencia de Tareas Atómicas (TDD)

- [ ] **T1. Escribir `afecta_uc_multiple_test.go` en rojo** con las funciones:
  - `TestEspecificadorAfectaUCList` (E1..E8, literal prohibido, línea de plantilla 3.1)
  - `TestClarificadorAfectaUCList` (C1..C9, con C9 ubicado tras el inicio del Hard Gate)
  - `TestDocumentadorAfectaUCList` (D1..D8)
  - `TestAfectaUCListDistribution` (install.sh en `t.TempDir()`; los literales E1, C1 y D5 deben leerse desde `.agents/skills/...` y `.claude/skills/...` del destino; ausencia de emojis en los cuatro archivos modificados con la misma regex de la 011)
  - `TestAfectaUCListBackwardCompatibility` (specs `001`..`013` presentes en `seb.lock` según corresponda conservan `verified` y `commit_hash`; las specs `001`..`012` conservan `Estado: APROBADA`)
  - _Hecho cuando:_ `go test -run 'AfectaUCList'` compila y falla por los literales ausentes.
- [ ] **T2. Actualizar `seb-especificador` (SKILL y plantilla)** — 014-RF-01..014-RF-04, 014-RF-13, 014-CL-01. _Hecho cuando:_ `014-CHK-01` pasa.
- [ ] **T3. Adaptar `TestEspecificadorAfectaUCDirectives` (014-RF-14)**: reemplazar exclusivamente la aserción de `"more than one independent `UC-<dominio>_<accion>`"` por la del literal E6 y la de la línea de plantilla anterior por la línea 3.1. _Hecho cuando:_ `014-CHK-04` y `014-CHK-05` pasan.
- [ ] **T4. Actualizar `seb-clarificador`** — 014-RF-05..014-RF-07, 014-CL-02..014-CL-06. _Hecho cuando:_ `014-CHK-02` pasa.
- [ ] **T5. Actualizar `seb-documentador`** — 014-RF-08..014-RF-12, 014-CL-07. _Hecho cuando:_ `014-CHK-03` pasa.
- [ ] **T6. Regresión completa** — 014-RF-16, 014-RF-17, 014-RNF-01, 014-RNF-02. _Hecho cuando:_ `014-CHK-06`, `014-CHK-07`, `014-CHK-98` y `014-CHK-99` pasan.

## 6. Riesgos

- `TestDocumentadorSkillDefinitionAndRules` (Spec 010) valida tokens del skill documentador: al editarlo no deben eliminarse textos existentes, solo agregarse.
- Si la Spec 013 se retoma, sus tests podrían colisionar con los identificadores sin prefijo de esta spec; queda fuera de alcance (pausada).
