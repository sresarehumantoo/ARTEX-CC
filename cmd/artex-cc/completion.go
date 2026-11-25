package main

import (
	"fmt"
	"os"
)

func cmdCompletion(args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: artex-cc completion <bash|zsh|fish>")
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

const bashCompletion = `_artex_cc() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="serve init health tools identity version completion help"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
        return 0
    fi

    case "${COMP_WORDS[1]}" in
        serve)
            COMPREPLY=( $(compgen -W "--stdio --pg --proxy --data --listen --web --debug --require-identity --version" -- "${cur}") )
            ;;
        init)
            COMPREPLY=( $(compgen -W "--goal --description --pg --data --version" -- "${cur}") )
            ;;
        health)
            COMPREPLY=( $(compgen -W "--pg --format --timeout" -- "${cur}") )
            ;;
        tools)
            COMPREPLY=( $(compgen -W "--json" -- "${cur}") )
            ;;
        identity)
            if [[ ${COMP_CWORD} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "issue rotate show" -- "${cur}") )
                return 0
            fi
            case "${COMP_WORDS[2]}" in
                issue)
                    COMPREPLY=( $(compgen -W "--data --name --ttl --format --url --bearer-env" -- "${cur}") )
                    ;;
                rotate)
                    COMPREPLY=( $(compgen -W "--data" -- "${cur}") )
                    ;;
                show)
                    COMPREPLY=( $(compgen -W "--data --format" -- "${cur}") )
                    ;;
            esac
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
            ;;
    esac
    return 0
}
complete -F _artex_cc artex-cc
`

const zshCompletion = `#compdef artex-cc

_artex_cc() {
    local -a commands
    commands=(
        'serve:Start the MCP tool server'
        'init:Initialize a new engagement'
        'health:Check PostgreSQL connectivity'
        'tools:List the MCP tool catalog'
        'identity:Issue, rotate or inspect client identity tokens'
        'version:Print version'
        'completion:Generate shell completion script'
        'help:Show usage information'
    )

    _arguments -C \
        '1:command:->command' \
        '*::arg:->args'

    case $state in
        command)
            _describe -t commands 'artex-cc command' commands
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
                        '--require-identity[Refuse intent claims without a verified identity]' \
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
                tools)
                    _arguments '--json[Print JSON]'
                    ;;
                identity)
                    _arguments -C \
                        '1:identity command:((issue\:"Print a signed identity token" rotate\:"Replace identity.key, invalidating every token" show\:"Print the key ID or inspect a token"))' \
                        '*::arg:->identity_args'
                    case $words[1] in
                        issue)
                            _arguments \
                                '--data[Data directory]:dir:_files -/' \
                                '--name[Identity name]:name:' \
                                '--ttl[Token lifetime, at most 30d]:duration:' \
                                '--format[Output format]:format:(token header claude-json)' \
                                '--url[MCP endpoint URL for claude-json]:url:' \
                                '--bearer-env[Environment variable holding the bearer token]:var:'
                            ;;
                        rotate)
                            _arguments '--data[Data directory]:dir:_files -/'
                            ;;
                        show)
                            _arguments \
                                '--data[Data directory]:dir:_files -/' \
                                '--format[Output format]:format:(text json)' \
                                '1:token (- reads stdin):'
                            ;;
                    esac
                    ;;
                completion)
                    _arguments '1:shell:(bash zsh fish)'
                    ;;
            esac
            ;;
    esac
}

_artex_cc "$@"
`

const fishCompletion = `complete -c artex-cc -e
complete -c artex-cc -n '__fish_use_subcommand' -a serve -d 'Start the MCP tool server'
complete -c artex-cc -n '__fish_use_subcommand' -a init -d 'Initialize a new engagement'
complete -c artex-cc -n '__fish_use_subcommand' -a health -d 'Check PostgreSQL connectivity'
complete -c artex-cc -n '__fish_use_subcommand' -a tools -d 'List the MCP tool catalog'
complete -c artex-cc -n '__fish_seen_subcommand_from tools' -l json -d 'Print JSON'
complete -c artex-cc -n '__fish_use_subcommand' -a version -d 'Print version'
complete -c artex-cc -n '__fish_use_subcommand' -a completion -d 'Generate shell completion script'
complete -c artex-cc -n '__fish_use_subcommand' -a help -d 'Show usage information'

complete -c artex-cc -n '__fish_seen_subcommand_from serve' -l stdio -d 'Use stdio transport instead of HTTP'
complete -c artex-cc -n '__fish_seen_subcommand_from serve' -l pg -d 'PostgreSQL connection string' -r
complete -c artex-cc -n '__fish_seen_subcommand_from serve' -l proxy -d 'Start recording proxy on ADDR' -r
complete -c artex-cc -n '__fish_seen_subcommand_from serve' -l data -d 'Data directory' -r -F
complete -c artex-cc -n '__fish_seen_subcommand_from serve' -l listen -d 'HTTP listen address' -r
complete -c artex-cc -n '__fish_seen_subcommand_from serve' -l web -d 'Enable web UI and REST API'
complete -c artex-cc -n '__fish_seen_subcommand_from serve' -l debug -d 'Enable debug-level logging'
complete -c artex-cc -n '__fish_seen_subcommand_from serve' -l require-identity -d 'Refuse intent claims without a verified identity'
complete -c artex-cc -n '__fish_seen_subcommand_from serve' -l version -d 'Print version and exit'

complete -c artex-cc -n '__fish_seen_subcommand_from init' -l goal -d 'Engagement goal' -r
complete -c artex-cc -n '__fish_seen_subcommand_from init' -l description -d 'Target description' -r
complete -c artex-cc -n '__fish_seen_subcommand_from init' -l pg -d 'PostgreSQL connection string' -r
complete -c artex-cc -n '__fish_seen_subcommand_from init' -l data -d 'Data directory' -r -F
complete -c artex-cc -n '__fish_seen_subcommand_from init' -l version -d 'Print version and exit'

complete -c artex-cc -n '__fish_seen_subcommand_from health' -l pg -d 'PostgreSQL connection string' -r
complete -c artex-cc -n '__fish_seen_subcommand_from health' -l format -d 'Output format (text or json)' -r -a 'text json'
complete -c artex-cc -n '__fish_seen_subcommand_from health' -l timeout -d 'Health check timeout' -r

complete -c artex-cc -n '__fish_use_subcommand' -a identity -d 'Issue, rotate or inspect client identity tokens'
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and not __fish_seen_subcommand_from issue rotate show' -a issue -d 'Print a signed identity token'
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and not __fish_seen_subcommand_from issue rotate show' -a rotate -d 'Replace identity.key, invalidating every token'
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and not __fish_seen_subcommand_from issue rotate show' -a show -d 'Print the key ID or inspect a token'
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and __fish_seen_subcommand_from issue rotate show' -l data -d 'Data directory' -r -F
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and __fish_seen_subcommand_from issue' -l name -d 'Identity name' -r
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and __fish_seen_subcommand_from issue' -l ttl -d 'Token lifetime, at most 30d' -r
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and __fish_seen_subcommand_from issue' -l format -d 'Output format' -r -a 'token header claude-json'
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and __fish_seen_subcommand_from issue' -l url -d 'MCP endpoint URL for claude-json' -r
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and __fish_seen_subcommand_from issue' -l bearer-env -d 'Environment variable holding the bearer token' -r
complete -c artex-cc -n '__fish_seen_subcommand_from identity; and __fish_seen_subcommand_from show' -l format -d 'Output format (text or json)' -r -a 'text json'

complete -c artex-cc -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
`
