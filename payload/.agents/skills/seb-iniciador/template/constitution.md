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
