# Spec <NNN> — <Título Tarea de la>

- **Tarea ID:** `<NNN>`
- **Identificador:** `<nombre_tarea>`
- **Origen:** [Prosa / Jira: KEY-123 / GitHub Issue: #123]
- **Estado:** `BORRADOR`

## 1. Contexto y Objetivo (El POR QUÉ)

[Un párrafo conciso explicando qué problema de negocio o usuario resuelve esta tarea y el valor que aporta.]

## 2. Actores y Usuarios

- **[Actor 1]:** [Rol o sistema consumidor que interactúa con la funcionalidad]

## 3. Historias de Usuario

- **HU-01:** Como `<rol>`, quiero `<acción>` para `<beneficio>`.

## 4. Definiciones de Dominio (Opcional)

- **[Término]:** [Definición precisa e inequívoca si existe riesgo de ambigüedad]

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

- **RF-01 (Event-Driven):** CUANDO <evento>, EL SISTEMA DEBE <respuesta observable y medible>.
- **RF-02 (Unwanted Behavior):** SI <condición de error o entrada inválida>, ENTONCES EL SISTEMA DEBE <respuesta exacta>.
- **RF-03 (State-Driven):** MIENTRAS <estado>, EL SISTEMA DEBE <respuesta exacta>.
- **RF-04 (Ubiquitous):** EL SISTEMA DEBE <invariante de comportamiento>.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01:** [Rendimiento, límites de tiempo, idempotencia, seguridad o formato de datos con umbral medible]

## 7. Casos Límite (Edge Cases)

- **CL-01 (Valores vacíos / nulos / duplicados):** [Comportamiento esperado]
- **CL-02 (Límites numéricos / paginación / concurrencia):** [Comportamiento esperado]

## 8. Fuera de Alcance (Out of Scope)

- [Lo que explícitamente NO se construirá ni modificará en esta iteración]

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`RF-01` a `RF-XX`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify <nombre_tarea>`.
- Cero violaciones a `docs/constitution.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

- [NECESITA ACLARACIÓN] [Pregunta concreta sobre cualquier vacío lógico, caso no contemplado en el ticket original o posible contradicción con la constitución. Si no hay ninguna, escribir: "Ninguna duda pendiente."]
