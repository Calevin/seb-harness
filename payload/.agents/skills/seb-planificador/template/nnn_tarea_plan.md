# Plan Técnico — <nombre_tarea> (Spec <NNN>)

- **Spec Origen:** `specs/<NNN>_<nombre_tarea>_spec.md`
- **Verificador Asociado:** `specs/<NNN>_<nombre_tarea>_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

[Explicación concisa del diseño. Cómo se separa la lógica de dominio pura (testeable sin infraestructura pesada) de la capa de transporte/UI/persistencia, respetando `docs/constitution.md`.]

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica                         | Requisitos Cubiertos (`RF-XX`) |
| :------------- | :----------------------------- | :---------------------------------------------- | :----------------------------- |
| `path/to/file` | `MODIFICAR`                    | [Descripción concisa]                           | `RF-01`, `RF-02`               |
| `path/to/test` | `CREAR`                        | [Tests unitarios/integración de los casos EARS] | `RF-01`, `RF-02`               |

## 3. Contratos, Firmas y Modelo de Datos

[Interfaces, structs/clases, firmas de funciones puras (inyectando dependencias como reloj/tiempo o repositorios por parámetro) y pseudocódigo del algoritmo crítico. Sin implementar el código final completo.]

## 4. Decisiones Técnicas Justificadas

- **Decisión 1:** [Qué se eligió]
  - _Alternativa descartada:_ [Qué otra opción existía y por qué se descartó según `constitution.md` o `MEMORY.md`].

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

_Regla para el Agente Implementador: Ejecuta UNA tarea a la vez en orden de dependencia. Primero escribe el test y verifica que falle (Rojo), luego escribe el código mínimo hasta pasar (Verde)._

- [ ] **T1. <Descripción 20-30 atómica de min máximo>** (`RF-XX`)
  - _Archivos:_ `...`
  - _Hecho cuando:_ El comando `CHK-01` de `specs/<NNN>_<nombre_tarea>_verificador.md` devuelve exit code `0`.
- [ ] **T2. <Descripción atómica>** (`RF-YY`)
  - _Archivos:_ `...`
  - _Hecho cuando:_ El comando `CHK-02` de `specs/<NNN>_<nombre_tarea>_verificador.md` devuelve exit code `0`.
