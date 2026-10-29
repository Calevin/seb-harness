# Verificador Programático — Gestión Determinista de Roles de Sesión y Aislamiento de Habilidades (Spec 008)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify estado_sesion_determinista` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01,RF-02,RF-03,RF-04,CL-01,CL-03 | Gestión de roles CLI (set, get, clear), normalización de mayúsculas/espacios, salida 1 ante roles no reconocidos e idempotencia | `direnv exec . bash -c "cd engine && go test -v -run TestRoleCLIManagement ./cmd/seb"` |
| CHK-02 | RF-05,RF-06,CL-02 | Jerarquía de precedencia de 4 niveles (SEB_SKILL > ACTIVE_ROLE > transcripción seb-* > implementador) y tolerancia a archivo corrupto | `direnv exec . bash -c "cd engine && go test -v -run TestActiveRolePrecedenceAndHierarchy ./cmd/seb"` |
| CHK-03 | RF-07,RF-08 | Aislamiento y herencia de permisos de implementador para skills de dominio externos (java-springboot) y protección de contratos | `direnv exec . bash -c "cd engine && go test -v -run TestRolePermissionsAndDomainSkillIsolation ./cmd/seb"` |
| CHK-04 | RF-09,CL-04 | Transición atómica a implementador en plan-lock y limpieza automática de ACTIVE_ROLE en comandos terminales verify e init | `direnv exec . bash -c "cd engine && go test -v -run TestRoleLifecyclePlanLockAndCleanup ./cmd/seb"` |
| CHK-98 | RNF-01,RNF-02,RF-Stack | Análisis estático oficial de Go (go vet) y verificación de latencia sub-5ms | `direnv exec . bash -c "cd engine && go vet ./..."` |
| CHK-99 | RF-GLOBAL | Compilación limpia de binario seb y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify estado_sesion_determinista` (o `008`) reporta **100% de los controles (`CHK-01` a `CHK-99`) en estado `[PASA]`**.
