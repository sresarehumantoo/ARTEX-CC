package main

import (
	"fmt"
	"os"
)

func cmdCompletion(args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: artex completion <bash|zsh|fish>")
		os.Exit(1)
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "unsupported shell: %s (use bash, zsh, or fish)\n", args[0])
		os.Exit(1)
	}
}

const bashCompletion = `_artex() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="serve init health version completion help"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
        return 0
    fi

    case "${COMP_WORDS[1]}" in
        serve)
            COMPREPLY=( $(compgen -W "--stdio --pg --proxy --data --listen --web --debug --version" -- "${cur}") )
            ;;
        init)
            COMPREPLY=( $(compgen -W "--goal --description --pg --data --version" -- "${cur}") )
            ;;
        health)
            COMPREPLY=( $(compgen -W "--pg --format --timeout" -- "${cur}") )
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
            ;;
    esac
    return 0
}
complete -F _artex artex
`

const zshCompletion = `#compdef artex

_artex() {
    local -a commands
    commands=(
        'serve:Start the MCP tool server'
        'init:Initialize a new engagement'
        'health:Check PostgreSQL connectivity'
        'version:Print version'
        'completion:Generate shell completion script'
        'help:Show usage information'
    )

    _arguments -C \
        '1:command:->command' \
        '*::arg:->args'

    case $state in
        command)
            _describe -t commands 'artex command' commands
            ;;
        args)
            case $words[1] in
                serve)
                    _arguments \
                        '--stdio[Use stdio transport instead of HTTP]' \
                        '--pg[PostgreSQL connection string]:dsn:' \
                        '--proxy[Start recording proxy on ADDR]:addr:' \
                        '--data[Data directory]:dir:_files -/' \
                        '--listen[HTTP listen address]:addr:' \
                        '--web[Enable web UI and REST API]' \
                        '--debug[Enable debug-level logging]' \
                        '--version[Print version and exit]'
                    ;;
                init)
                    _arguments \
                        '--goal[Engagement goal]:goal:' \
                        '--description[Target description]:desc:' \
                        '--pg[PostgreSQL connection string]:dsn:' \
                        '--data[Data directory]:dir:_files -/' \
                        '--version[Print version and exit]'
                    ;;
                health)
                    _arguments \
                        '--pg[PostgreSQL connection string]:dsn:' \
                        '--format[Output format]:format:(text json)' \
                        '--timeout[Health check timeout]:duration:'
                    ;;
                completion)
                    _arguments '1:shell:(bash zsh fish)'
                    ;;
            esac
            ;;
    esac
}

_artex "$@"
`

const fishCompletion = `complete -c artex -e
complete -c artex -n '__fish_use_subcommand' -a serve -d 'Start the MCP tool server'
complete -c artex -n '__fish_use_subcommand' -a init -d 'Initialize a new engagement'
complete -c artex -n '__fish_use_subcommand' -a health -d 'Check PostgreSQL connectivity'
complete -c artex -n '__fish_use_subcommand' -a version -d 'Print version'
complete -c artex -n '__fish_use_subcommand' -a completion -d 'Generate shell completion script'
complete -c artex -n '__fish_use_subcommand' -a help -d 'Show usage information'

complete -c artex -n '__fish_seen_subcommand_from serve' -l stdio -d 'Use stdio transport instead of HTTP'
complete -c artex -n '__fish_seen_subcommand_from serve' -l pg -d 'PostgreSQL connection string' -r
complete -c artex -n '__fish_seen_subcommand_from serve' -l proxy -d 'Start recording proxy on ADDR' -r
complete -c artex -n '__fish_seen_subcommand_from serve' -l data -d 'Data directory' -r -F
complete -c artex -n '__fish_seen_subcommand_from serve' -l listen -d 'HTTP listen address' -r
complete -c artex -n '__fish_seen_subcommand_from serve' -l web -d 'Enable web UI and REST API'
complete -c artex -n '__fish_seen_subcommand_from serve' -l debug -d 'Enable debug-level logging'
complete -c artex -n '__fish_seen_subcommand_from serve' -l version -d 'Print version and exit'

complete -c artex -n '__fish_seen_subcommand_from init' -l goal -d 'Engagement goal' -r
complete -c artex -n '__fish_seen_subcommand_from init' -l description -d 'Target description' -r
complete -c artex -n '__fish_seen_subcommand_from init' -l pg -d 'PostgreSQL connection string' -r
complete -c artex -n '__fish_seen_subcommand_from init' -l data -d 'Data directory' -r -F
complete -c artex -n '__fish_seen_subcommand_from init' -l version -d 'Print version and exit'

complete -c artex -n '__fish_seen_subcommand_from health' -l pg -d 'PostgreSQL connection string' -r
complete -c artex -n '__fish_seen_subcommand_from health' -l format -d 'Output format (text or json)' -r -a 'text json'
complete -c artex -n '__fish_seen_subcommand_from health' -l timeout -d 'Health check timeout' -r

complete -c artex -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
`
