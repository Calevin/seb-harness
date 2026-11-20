package main

import (
	"fmt"
	"io"
	"strings"
)

// commandHelpSpec describe la ficha tecnica completa de un subcomando del CLI seb.
type commandHelpSpec struct {
	Name            string
	Summary         string
	Syntax          string
	Flags           []string
	Reads           []string
	Writes          []string
	Preconditions   []string
	StateTransition string
	ExitCodes       []string
}

// commandCatalog mantiene el orden canonico de los subcomandos activos en seb.
var commandCatalog = []commandHelpSpec{
	{
		Name:    "init",
		Summary: "Inicializa directorios de gobernanza y la base SQLite .seb/seb-audit.db",
		Syntax:  "seb init",
		Flags:   []string{"Ninguno"},
		Reads:   []string{"Estado actual del workspace"},
		Writes:  []string{".seb/bin", ".agents/rules", ".agents/skills/*", ".seb/seb-audit.db (esquema SQLite)", ".seb/ACTIVE_ROLE (limpia rol activo)"},
		Preconditions: []string{
			"Permisos de escritura en el directorio de trabajo actual.",
		},
		StateTransition: "Ninguna sobre tareas; inicializa el entorno local de auditoria.",
		ExitCodes: []string{
			"0: Inicializacion completada correctamente.",
			"1: Error creando directorios o inicializando .seb/seb-audit.db.",
		},
	},
	{
		Name:    "spec-next",
		Summary: "Reserva el siguiente ID secuencial (<NNN>) y crea el stub de spec en specs/",
		Syntax:  "seb spec-next <nombre_tarea>",
		Flags:   []string{"Ninguno"},
		Reads:   []string{"specs/*_spec.md", "tasks (SQLite)"},
		Writes:  []string{"specs/<NNN>_<nombre_tarea>_spec.md", "tasks (SQLite)", ".seb/ACTIVE_TASK"},
		Preconditions: []string{
			"Rol permitido: seb-especificador (en guardarrailes SDD).",
			"<nombre_tarea> saneado sin path traversal ni colision de identificador.",
		},
		StateTransition: "Markdown: (nuevo) -> BORRADOR | Motor (SQLite): (nuevo) -> SPECIFIED",
		ExitCodes: []string{
			"0: Reserva exitosa; imprime la ruta exacta de la spec creada.",
			"1: Nombre invalido, colision de tarea o error de escritura/SQLite.",
		},
	},
	{
		Name:    "plan-lock",
		Summary: "Valida spec/plan/verificador, dependencias DAG y sella hashes SHA-256 en seb.lock",
		Syntax:  "seb plan-lock <tarea>",
		Flags:   []string{"Ninguno (acepta ID, nombre simple o clave canonica)"},
		Reads:   []string{"specs/<NNN>_<tarea>_spec.md", "specs/<NNN>_<tarea>_plan.md", "specs/<NNN>_<tarea>_verificador.md", ".seb/seb.lock", "tasks (SQLite)"},
		Writes:  []string{".seb/seb.lock (spec_hash, verifier_hash)", "tasks (SQLite: status=PLANNED)", ".seb/ACTIVE_TASK"},
		Preconditions: []string{
			"Rol permitido: seb-planificador.",
			"La spec debe existir, no estar en BORRADOR/ABORTADA/BLOQUEADA_POR_DEPENDENCIA y tener cero etiquetas [NECESITA ACLARACION].",
			"Deben existir plan y verificador con matriz de checks valida (<NNN>-CHK-*) y sin dependencias incumplidas ni ciclos en el DAG.",
		},
		StateTransition: "Markdown: APROBADA (sin cambios) | Motor (SQLite): SPECIFIED -> PLANNED | seb.lock: registra spec_hash y verifier_hash",
		ExitCodes: []string{
			"0: Plan sellado correctamente en .seb/seb.lock.",
			"1: Bloqueo SDD por aclaraciones pendientes, dependencia incumplida, ciclo DAG o verificador invalido.",
		},
	},
	{
		Name:    "verify",
		Summary: "Ejecuta de forma determinista specs/<NNN>_<tarea>_verificador.md y sella el resultado",
		Syntax:  "seb verify <tarea>",
		Flags:   []string{"Ninguno (acepta ID, nombre simple o clave canonica)"},
		Reads:   []string{"specs/<NNN>_<tarea>_spec.md", "specs/<NNN>_<tarea>_verificador.md", ".seb/seb.lock", "git rev-parse HEAD"},
		Writes:  []string{"verifications (SQLite)", "tasks (SQLite: VERIFIED o FAILED_VERIFICATION)", ".seb/seb.lock (verified, commit_hash)"},
		Preconditions: []string{
			"La tarea no debe estar abortada.",
			"Los hashes SHA-256 actuales de spec y verificador deben coincidir con los sellados por plan-lock en .seb/seb.lock.",
			"Cada fila <NNN>-CHK-* del verificador debe retornar exit code 0.",
		},
		StateTransition: "Motor (SQLite): PLANNED -> VERIFIED (exito) o FAILED_VERIFICATION (fallo) | seb.lock: verified=true y commit_hash en exito",
		ExitCodes: []string{
			"0: Todos los checks retornaron 0 y la verificacion quedo sellada.",
			"1: Al menos un check fallo, hash alterado (tampering) o verificador malformado.",
		},
	},
	{
		Name:    "status",
		Summary: "Muestra el estado unificado (specs/, seb.lock y SQLite), alertas y siguiente paso",
		Syntax:  "seb status [<tarea>] [--compact] [--json]",
		Flags: []string{
			"<tarea>     Opcional: filtra e inspecciona una sola tarea por ID, nombre o clave canonica.",
			"--compact   Muestra la tabla resumida minima.",
			"--json      Emite la salida estructurada completa en formato JSON valido.",
		},
		Reads:   []string{"specs/*_spec.md", "specs/*_plan.md", "specs/*_verificador.md", ".seb/seb.lock", "tasks (SQLite, opcional si existe)"},
		Writes:  []string{"Ninguno (operacion estrictamente de solo lectura; no crea .seb/seb-audit.db)"},
		Preconditions: []string{
			"Permitido para todos los roles SDD.",
			"Funciona en clones limpios sin .seb/seb-audit.db.",
		},
		StateTransition: "Ninguna (solo lectura e introspeccion).",
		ExitCodes: []string{
			"0: Consulta completada (incluyendo repositorios vacios sin specs).",
			"1: Tarea solicitada inexistente o ambigua, o archivo .seb/seb.lock corrupto.",
		},
	},
	{
		Name:    "abort",
		Summary: "Aborta formalmente una tarea (ABORTED) o purga sus artefactos con --purge",
		Syntax:  "seb abort <tarea> [--purge] [--force|-f]",
		Flags: []string{
			"--purge      Elimina fisicamente spec/plan/verificador y registros en SQLite y seb.lock.",
			"--force, -f  Fuerza aborto/purga aun cuando existan tareas dependientes o estado verificado.",
		},
		Reads:   []string{"specs/*", ".seb/seb.lock", "tasks (SQLite)"},
		Writes:  []string{"specs/<NNN>_<tarea>_spec.md (Estado: ABORTADA) o eliminacion con --purge", ".seb/seb.lock", "tasks (SQLite)", ".seb/ACTIVE_TASK"},
		Preconditions: []string{
			"No debe tener dependientes activos en el DAG ni estar verificada salvo que se indique --force.",
		},
		StateTransition: "Markdown: -> ABORTADA | Motor (SQLite): -> ABORTED (o eliminacion con --purge) | seb.lock: aborted=true (o eliminacion)",
		ExitCodes: []string{
			"0: Tarea abortada o purgada correctamente.",
			"1: Tarea inexistente, ambigua o bloqueada por dependientes/verificacion sin --force.",
		},
	},
	{
		Name:    "task",
		Summary: "Espacio de nombres estructurado para gestion de tareas (alias: seb task abort)",
		Syntax:  "seb task abort <tarea> [--purge] [--force|-f]",
		Flags: []string{
			"--purge      Elimina artefactos y registros de la tarea.",
			"--force, -f  Omite bloqueos de dependientes activos o verificacion previa.",
		},
		Reads:           []string{"specs/*", ".seb/seb.lock", "tasks (SQLite)"},
		Writes:          []string{"Delegado a seb abort (spec, seb.lock, SQLite, ACTIVE_TASK)"},
		Preconditions:   []string{"Subcomando valido ('abort') y mismas reglas que seb abort."},
		StateTransition: "Identica a seb abort.",
		ExitCodes: []string{
			"0: Operacion completada.",
			"1: Subcomando desconocido o fallo de abort/purga.",
		},
	},
	{
		Name:    "guard",
		Summary: "Evalua permisos de menor privilegio por rol SDD sobre rutas o acciones (<1ms)",
		Syntax:  "seb guard <rol_o_skill> <write|exec> <ruta_o_comando>",
		Flags:   []string{"Ninguno"},
		Reads:   []string{"Reglas internas de menor privilegio por rol"},
		Writes:  []string{"agent_actions (SQLite: solo registra bloqueos DENIED)"},
		Preconditions: []string{
			"Requiere los 3 argumentos posicionales.",
		},
		StateTransition: "Ninguna sobre el ciclo SDD.",
		ExitCodes: []string{
			"0: Accion permitida para el rol.",
			"1: Accion denegada por guardrail de menor privilegio o argumentos insuficientes.",
		},
	},
	{
		Name:    "role",
		Summary: "Gestiona el rol de sesion activo en .seb/ACTIVE_ROLE",
		Syntax:  "seb role <set <rol>|get|clear>",
		Flags:   []string{"Ninguno"},
		Reads:   []string{".seb/ACTIVE_ROLE"},
		Writes:  []string{".seb/ACTIVE_ROLE"},
		Preconditions: []string{
			"El rol 'migrador' solo puede ser activado por el humano desde terminal (bloqueado para agentes en PreToolUse).",
		},
		StateTransition: "Ninguna sobre el ciclo SDD de tareas; actualiza el rol de sesion local.",
		ExitCodes: []string{
			"0: Subcomando ejecutado correctamente.",
			"1: Subcomando o rol invalido o error de I/O.",
		},
	},
	{
		Name:    "memory-add",
		Summary: "Registra un aprendizaje o restriccion en MEMORY.md y lo audita en SQLite",
		Syntax:  "seb memory-add \"<categoria>\" \"<descripcion>\"",
		Flags:   []string{"Ninguno"},
		Reads:   []string{"MEMORY.md", ".seb/ACTIVE_TASK"},
		Writes:  []string{"MEMORY.md", "agent_actions (SQLite)"},
		Preconditions: []string{
			"Requiere categoria y descripcion no vacias.",
		},
		StateTransition: "Ninguna sobre el estado de la tarea.",
		ExitCodes: []string{
			"0: Entrada agregada a MEMORY.md y auditada.",
			"1: Argumentos faltantes o error de escritura en MEMORY.md.",
		},
	},
	{
		Name:    "audit",
		Summary: "Registra telemetria, importa usage.jsonl y genera reportes de latencia/costos",
		Syntax:  "seb audit <log|import-usage|report> [flags]",
		Flags: []string{
			"log           Registra invocacion, tokens, duracion y costo USD.",
			"import-usage  Importa registros desde un archivo usage.jsonl.",
			"report        Imprime resumen consolidado por tarea.",
		},
		Reads:           []string{"agent_actions (SQLite)", "verifications (SQLite)", ".seb/config.json", "usage.jsonl (en import-usage)"},
		Writes:          []string{"agent_actions (SQLite en log/import-usage)"},
		Preconditions:   []string{"Base de auditoria inicializada."},
		StateTransition: "Ninguna sobre el estado SDD de tareas.",
		ExitCodes: []string{
			"0: Subcomando de auditoria ejecutado.",
			"1: Subcomando desconocido, flags invalidos o error de SQLite.",
		},
	},
	{
		Name:    "hook",
		Summary: "Procesa eventos de ciclo de vida del agente (PreToolUse, PostToolUse, Stop)",
		Syntax:  "seb hook <pre-tool|post-tool|stop> [--format antigravity|claude]",
		Flags: []string{
			"--format  Selecciona el protocolo de salida (antigravity por defecto o claude).",
		},
		Reads:           []string{"stdin (payload JSON del evento)", ".seb/ACTIVE_ROLE", ".seb/ACTIVE_TASK", ".seb/config.json"},
		Writes:          []string{"agent_actions (SQLite)", "archivos temporales de timer en /tmp"},
		Preconditions:   []string{"Invocado automaticamente por el runtime del IDE/agente con JSON valido en stdin."},
		StateTransition: "Ninguna sobre el estado SDD de tareas.",
		ExitCodes: []string{
			"0: Evaluacion completada (allow/deny en stdout segun formato).",
			"1: Subcomando de hook invalido o fallo fatal de lectura.",
		},
	},
	{
		Name:    "oneoff",
		Summary: "Lista o ejecuta operaciones one-off de migracion del arnes (solo rol migrador)",
		Syntax:  "seb oneoff [<id_spec>] [--dry-run]",
		Flags: []string{
			"--dry-run  Simula la migracion sin modificar archivos ni sellos.",
		},
		Reads:           []string{"specs/*", ".seb/seb.lock", ".seb/ACTIVE_ROLE", "docs/use-cases/*"},
		Writes:          []string{"specs/*_spec.md", "specs/*_plan.md", "specs/*_verificador.md", ".seb/seb.lock", "agent_actions (SQLite)"},
		Preconditions:   []string{"Para ejecutar (sin solo listar): rol activo 'migrador' activado manualmente por el humano."},
		StateTransition: "Reescribe y resella artefactos historicos manteniendo sus hashes sincronizados en .seb/seb.lock.",
		ExitCodes: []string{
			"0: Listado o ejecucion one-off completada.",
			"1: Rol migrador ausente al intentar ejecutar, ID desconocido o fallo de migracion.",
		},
	},
	{
		Name:    "help",
		Summary: "Muestra el catalogo de comandos o la ficha tecnica detallada de un subcomando",
		Syntax:  "seb help [<subcomando>]",
		Flags:   []string{"Tambien disponible mediante seb <subcomando> --help o -h."},
		Reads:   []string{"Catalogo interno de comandos del binario seb"},
		Writes:  []string{"Ninguno (solo lectura, sin tocar .seb/seb-audit.db)"},
		Preconditions: []string{
			"Permitido para todos los roles SDD sin restricciones.",
		},
		StateTransition: "Ninguna.",
		ExitCodes: []string{
			"0: Catalogo o ficha tecnica mostrada correctamente.",
			"1: Subcomando solicitado inexistente.",
		},
	},
}

// hasHelpFlag indica si entre los argumentos aparece --help o -h.
func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// findCommandHelp busca una ficha tecnica por nombre de subcomando.
func findCommandHelp(name string) (commandHelpSpec, bool) {
	clean := strings.TrimSpace(strings.ToLower(name))
	for _, spec := range commandCatalog {
		if spec.Name == clean {
			return spec, true
		}
	}
	return commandHelpSpec{}, false
}

// renderHelpCatalog imprime el catalogo compacto de todos los subcomandos disponibles.
func renderHelpCatalog(w io.Writer) {
	fmt.Fprintln(w, "seb — Motor determinista SDD y Auditoria SQLite (seb-harness)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Comandos disponibles:")
	for _, spec := range commandCatalog {
		fmt.Fprintf(w, "  %-14s %s\n", spec.Name, spec.Summary)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Uso bajo demanda: ejecuta 'seb help <subcomando>' o 'seb <subcomando> --help' para ver que lee, que escribe, precondiciones y transiciones.")
}

// renderCommandHelp imprime la ficha tecnica completa de un subcomando.
func renderCommandHelp(w io.Writer, spec commandHelpSpec) {
	fmt.Fprintf(w, "Subcomando: seb %s\n", spec.Name)
	fmt.Fprintf(w, "Proposito: %s\n", spec.Summary)
	fmt.Fprintf(w, "Sintaxis: %s\n", spec.Syntax)

	fmt.Fprintln(w, "Flags / Argumentos:")
	for _, f := range spec.Flags {
		fmt.Fprintf(w, "  - %s\n", f)
	}

	fmt.Fprintln(w, "Archivos y tablas que lee:")
	for _, r := range spec.Reads {
		fmt.Fprintf(w, "  - %s\n", r)
	}

	fmt.Fprintln(w, "Archivos y tablas que modifica:")
	for _, wr := range spec.Writes {
		fmt.Fprintf(w, "  - %s\n", wr)
	}

	fmt.Fprintln(w, "Precondiciones y guardarrailes:")
	for _, p := range spec.Preconditions {
		fmt.Fprintf(w, "  - %s\n", p)
	}

	fmt.Fprintf(w, "Transicion de estado: %s\n", spec.StateTransition)

	fmt.Fprintln(w, "Codigos de salida:")
	for _, e := range spec.ExitCodes {
		fmt.Fprintf(w, "  - %s\n", e)
	}
}

// runHelpCommand ejecuta el sistema de ayuda bajo demanda y retorna el codigo de salida.
func runHelpCommand(args []string, stdout, stderr io.Writer) int {
	// Si no hay argumentos o solo se pasa --help/-h, mostrar catalogo general
	var filtered []string
	for _, a := range args {
		if a == "--help" || a == "-h" {
			continue
		}
		filtered = append(filtered, a)
	}

	if len(filtered) == 0 {
		renderHelpCatalog(stdout)
		return 0
	}

	target := filtered[0]
	// Soportar 'seb help task abort' redirigiendo a la ficha de abort
	if target == "task" && len(filtered) > 1 && filtered[1] == "abort" {
		target = "abort"
	}

	spec, ok := findCommandHelp(target)
	if !ok {
		fmt.Fprintf(stderr, "[seb-harness ERROR] Subcomando desconocido para ayuda: '%s'. Ejecuta 'seb help' para ver el catalogo.\n", target)
		return 1
	}

	renderCommandHelp(stdout, spec)
	return 0
}
