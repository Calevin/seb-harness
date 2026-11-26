<!-- BEGIN seb-harness:managed -->
<!-- BLOQUE GESTIONADO POR seb-harness. NO EDITAR: se actualiza en cada install.sh -->
# Gobernanza del Proyecto (`seb-harness`)

Este repositorio opera bajo **`seb-harness`** (SDD + Verificación Determinista).

## Fuentes de Verdad Base
1. **`docs/constitution.md`**: Ley suprema de arquitectura, stack, dominio y estilo (si no existe, ejecuta `seb-iniciador`).
2. **`MEMORY.md`**: Callejones sin salida, restricciones técnicas y decisiones históricas.

## Enrutamiento Operativo
- **Flujo SDD completo** (features, cambios de arquitectura, contratos públicos, nuevas dependencias o riesgo medio/alto): **lee `.agents/SEB-HARNESS.md` antes de actuar** y delega el control al binario `.seb/bin/seb` (`status`, `help`, `spec-next`, `plan-lock`, `verify`, `oneoff`).
- **Flujo Rápido (`FAST`)** (solo cambios menores de bajo riesgo, `<= 4` archivos y `<= 100` líneas por defecto, sin cambios de arquitectura/contratos/dependencias): **se inicia únicamente invocando el skill `seb-fast-task`** (`.agents/skills/seb-fast-task/SKILL.md`), que genera 2 artefactos acotados sellados y verificados con `.seb/bin/seb`.
<!-- END seb-harness:managed -->

## Instrucciones Propias del Proyecto
<!-- Escribe aquí directivas específicas de tu proyecto. Todo contenido fuera de los marcadores se conserva intacto en cada actualización. -->
