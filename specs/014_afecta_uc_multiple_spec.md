# Spec 014 — Soporte de Múltiples Casos de Uso en el Metadato `Afecta-UC:`

- **Tarea ID:** `014`
- **Identificador:** `afecta_uc_multiple`
- **Origen:** Prosa (solicitud del usuario: "se requiere que ahora sea posible que `Afecta-UC` refiera a más de una spec por vez", interpretado como más de un caso de uso por especificación)
- **Estado:** `APROBADA`
- **Dependencias:** `010_integracion_documentador`, `011_gobernanza_afecta_uc`
- **Sustituye:** CL-03 y RF-03 de `011_gobernanza_afecta_uc`
- **Afecta-UC:** `UC-documentation_functional_generation`

## 1. Contexto y Objetivo (El POR QUÉ)

La Spec 011 estableció que el metadato `Afecta-UC:` admite exactamente un valor (`UC-<dominio>_<accion>`, `nuevo` o `ninguno`) e impuso, mediante su caso límite CL-03, que toda petición que impacte más de un caso de uso independiente se divida en especificaciones separadas. En la práctica, existen cambios transversales cohesivos (por ejemplo, una regla de gobernanza que altera simultáneamente la especificación, la clarificación y la documentación) cuya división artificial en varias specs genera fragmentación, dependencias cruzadas forzadas y duplicación de requisitos, sin aportar trazabilidad adicional.

Esta especificación relaja la cardinalidad de `Afecta-UC:` para que una misma especificación pueda declarar una lista de varios casos de uso afectados, manteniendo la validación referencial, la coherencia con los `RF-XX` y la compatibilidad con las especificaciones ya selladas que declaran un único valor. El objetivo es que la Documentación Viva de la Fase 6 actualice todos los casos de uso realmente impactados por una tarea sin obligar a fragmentar su especificación.

Por decisión del usuario, esta especificación sustituye formalmente el caso límite CL-03 y el requisito RF-03 (literal de plantilla) de la Spec 011. La Spec 013 (`identificadores_unicos_requisitos`) queda pausada hasta que esta especificación sea verificada, por lo que esta spec utiliza el formato de identificadores vigente (`RF-XX`, sin prefijo).

## 2. Actores y Usuarios

- **Agente Especificador (`seb-especificador`):** Declara en la cabecera uno o varios casos de uso afectados.
- **Agente Clarificador (`seb-clarificador`):** Audita la sintaxis de la lista, la existencia de cada identificador y la coherencia de cada elemento con los `RF-XX`.
- **Agente Documentador (`seb-documentador`):** Actualiza cada caso de uso existente declarado, crea los casos de uso nuevos que correspondan y registra la evolución en el catálogo durante la Fase 6.
- **Agente Implementador:** Adapta las aserciones de los tests de la Spec 011 que verifican literales sustituidos por esta especificación.
- **Desarrollador / Usuario Humano:** Decide si una petición multi-UC se mantiene en una sola spec o se divide.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador, quiero declarar varios casos de uso en `Afecta-UC:` cuando un cambio cohesivo impacta más de una funcionalidad, para no fragmentar artificialmente la especificación.
- **HU-02:** Como desarrollador, quiero que `seb-clarificador` valide cada identificador de la lista de forma individual, para impedir referencias rotas parciales.
- **HU-03:** Como desarrollador, quiero que `seb-documentador` actualice todos los casos de uso declarados y registre qué specs los hicieron evolucionar, para que la Documentación Viva refleje el impacto completo y la historia de cada caso de uso.
- **HU-04:** Como mantenedor del arnés, quiero que las specs con un único valor sigan siendo válidas sin modificación, para no invalidar sellos ni el grafo de dependencias.

## 4. Definiciones de Dominio

- **Valor Simple de `Afecta-UC:`:** Un único elemento delimitado entre backticks: `` `UC-<dominio>_<accion>` ``, `` `nuevo` `` o `` `ninguno` `` (formato vigente según la Spec 011).
- **Lista de `Afecta-UC:`:** Dos o más elementos, cada uno delimitado individualmente entre backticks y separados por coma y espacio (`, `), por ejemplo: `` `UC-a_b`, `UC-c_d` `` o `` `UC-a_b`, `nuevo` ``. El orden de los elementos carece de significado.
- **Valor Exclusivo:** El literal `ninguno`, que no puede combinarse con ningún otro elemento.
- **Elemento `nuevo` en Lista:** Aparece como máximo una vez y representa uno o más casos de uso nuevos cuya cantidad e identificadores determina `seb-documentador` a partir de los `RF-XX`.
- **Columna "Specs de Evolución":** Columna del catálogo `docs/use-cases/README.md` que enumera las especificaciones posteriores a la Spec Originaria que modificaron el caso de uso.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

### Fase 1: Especificación y Plantilla (`seb-especificador`)
- **RF-01 (Ubiquitous):** EL SISTEMA DEBE aceptar en la cabecera de toda nueva especificación el campo `- **Afecta-UC:**` con un Valor Simple o con una Lista de `Afecta-UC:`.
- **RF-02 (Ubiquitous):** EL SISTEMA DEBE exigir que cada elemento de una Lista de `Afecta-UC:` sea un identificador `` `UC-<dominio>_<accion>` `` o el literal `` `nuevo` ``, cada uno delimitado individualmente entre backticks, separado del siguiente por coma y espacio, con `nuevo` presente como máximo una vez y admitiendo la combinación de `nuevo` con identificadores existentes.
- **RF-03 (Ubiquitous):** EL SISTEMA DEBE reflejar en la plantilla base de especificaciones de `seb-especificador` que el campo `Afecta-UC:` admite un Valor Simple o una Lista, mostrando la sintaxis de lista.
- **RF-04 (Event-Driven):** CUANDO una petición impacte más de un caso de uso, EL SISTEMA DEBE instruir a `seb-especificador` declarar todos los casos de uso afectados en una Lista de `Afecta-UC:` si los cambios son cohesivos, o sugerir la división en especificaciones separadas si los cambios son funcionalmente independientes.

### Fase 2: Auditoría en Clarificación (`seb-clarificador`)
- **RF-05 (Event-Driven):** CUANDO `seb-clarificador` audite una especificación cuyo `Afecta-UC:` sea una Lista, EL SISTEMA DEBE instruirle validar la sintaxis de la lista según RF-02 y la existencia de cada identificador `UC-<dominio>_<accion>` en `docs/use-cases/README.md`, sin considerar el orden de los elementos.
- **RF-06 (Event-Driven):** CUANDO `seb-clarificador` audite los `RF-XX` de una especificación con Lista de `Afecta-UC:`, EL SISTEMA DEBE instruirle verificar que cada elemento declarado esté respaldado por al menos un `RF-XX` y que ningún `RF-XX` modifique un caso de uso catalogado ausente de la lista.
- **RF-07 (Unwanted Behavior):** SI una Lista de `Afecta-UC:` contiene el literal `ninguno`, un elemento sin backticks, un separador distinto de coma y espacio, un elemento duplicado (incluido `nuevo` repetido) o un identificador `UC-*` inexistente en `docs/use-cases/README.md`, ENTONCES EL SISTEMA DEBE exigir que `seb-clarificador` formule la observación como pregunta directa sin editar la especificación previamente y prohíba la transición a `APROBADA` hasta su resolución.

### Fase 6: Documentación (`seb-documentador`)
- **RF-08 (Event-Driven):** CUANDO `seb-documentador` procese una especificación cuyo `Afecta-UC:` sea una Lista, EL SISTEMA DEBE instruirle actualizar cada caso de uso `UC-<dominio>_<accion>` declarado.
- **RF-09 (Event-Driven):** CUANDO `seb-documentador` procese una especificación cuyo `Afecta-UC:` contenga `nuevo` (como Valor Simple o dentro de una Lista), EL SISTEMA DEBE instruirle determinar a partir de los `RF-XX` la cantidad de casos de uso nuevos y asignar a cada uno un identificador `UC-<dominio>_<accion>` inexistente en el catálogo.
- **RF-10 (Ubiquitous):** EL SISTEMA DEBE instruir a `seb-documentador` mantener en la tabla índice de `docs/use-cases/README.md` una columna "Specs de Evolución" ubicada inmediatamente después de "Spec Originaria".
- **RF-11 (Event-Driven):** CUANDO `seb-documentador` actualice un caso de uso existente a partir de una especificación, EL SISTEMA DEBE instruirle agregar la ruta de dicha especificación a la columna "Specs de Evolución" de la fila de ese caso de uso, sin modificar su "Spec Originaria".
- **RF-12 (Unwanted Behavior):** SI `docs/use-cases/README.md` existe sin la columna "Specs de Evolución", ENTONCES EL SISTEMA DEBE instruir a `seb-documentador` agregarla y completar con `-` las filas que no tengan specs de evolución registradas.

### Sustitución de la Spec 011
- **RF-13 (Ubiquitous):** EL SISTEMA DEBE reemplazar en las directivas de `seb-especificador` la regla de división obligatoria multi-UC del caso límite CL-03 de la Spec 011 por el criterio de RF-04.
- **RF-14 (Ubiquitous):** EL SISTEMA DEBE permitir al Agente Implementador adaptar exclusivamente las aserciones de los tests del motor vinculados a `CHK-01` de la Spec 011 que comprueban el literal de división multi-UC y el literal de plantilla `` - **Afecta-UC:** [`UC-<dominio>_<accion>` | `nuevo` | `ninguno`] ``, sustituyéndolas por aserciones equivalentes sobre los literales nuevos, sin eliminar ni relajar ninguna otra aserción.
- **RF-15 (State-Driven):** MIENTRAS la Spec 011 permanezca sellada en `.seb/seb.lock`, EL SISTEMA DEBE conservar sin modificación su spec, su verificador y su sello, y `.seb/bin/seb verify gobernanza_afecta_uc` DEBE seguir retornando código de salida `0` tras la adaptación de RF-14.

### Alcance, Distribución y Compatibilidad
- **RF-16 (Ubiquitous):** EL SISTEMA DEBE implementar las reglas de esta especificación exclusivamente en las directivas de habilidades y plantillas distribuibles de `payload/` y en los tests indicados en RF-14, sin agregar ni modificar lógica de parseo de `Afecta-UC:` en el binario Go del motor.
- **RF-17 (State-Driven):** MIENTRAS existan especificaciones previas a esta con un Valor Simple en `Afecta-UC:`, EL SISTEMA DEBE considerarlas válidas sin exigir su migración ni alterar su sello en `.seb/seb.lock`.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Consistencia de Distribución):** Los archivos modificados en `payload/.agents/` deben ser accesibles de forma idéntica desde `.agents/` y no deben contener emojis; los proyectos destino aprovisionados por `install.sh` (incluido el destino `.claude/` de la Spec 012) deben recibir las directivas actualizadas.
- **RNF-02 (Cero Regresión del Motor):** La compilación, `go vet` y la suite de pruebas del motor Go deben pasar sin regresiones tras el cambio.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Lista con un único elemento):** Una Lista con un solo elemento es equivalente a un Valor Simple y debe considerarse válida.
- **CL-02 (`ninguno` combinado):** `` `ninguno`, `UC-a_b` `` es inválido; se aplica RF-07.
- **CL-03 (Elementos duplicados):** `` `UC-a_b`, `UC-a_b` `` es inválido; se aplica RF-07.
- **CL-04 (Múltiples `nuevo`):** `` `nuevo`, `nuevo` `` es inválido; se aplica RF-07. Un único `nuevo` puede originar uno o más casos de uso según RF-09.
- **CL-05 (Catálogo ausente):** Si `docs/use-cases/README.md` no existe y la Lista contiene algún `UC-*`, cada referencia se trata como inexistente (comportamiento heredado del CL-01 de la Spec 011).
- **CL-06 (Orden distinto):** `` `UC-c_d`, `UC-a_b` `` y `` `UC-a_b`, `UC-c_d` `` son equivalentes y ambos válidos.
- **CL-07 (Spec ya registrada como evolución):** Si la ruta de la especificación ya figura en "Specs de Evolución" de un caso de uso, `seb-documentador` no debe duplicarla.

## 8. Fuera de Alcance (Out of Scope)

- Validación sintáctica de `Afecta-UC:` en el binario Go (`plan-lock`, `verify`, `guard`).
- Migración retroactiva de specs `001` a `013` al formato de lista.
- Modificación de la spec, el verificador o el sello de la Spec 011.
- Reconstrucción histórica de la columna "Specs de Evolución" para specs anteriores a esta.
- Cambios en la estructura interna de los archivos `docs/use-cases/UC-<dominio>_<accion>.md` definida en la Spec 010.
- Adopción del formato de identificadores prefijados de la Spec 013 (pausada).

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`RF-01` a `RF-17` y `RNF-01` a `RNF-02`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify afecta_uc_multiple`.
- `.seb/bin/seb verify gobernanza_afecta_uc` continúa en `[PASA]`.
- Cero violaciones a `docs/constitution.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente.
