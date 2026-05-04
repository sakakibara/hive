package workspace

import "fmt"

// ShellInit returns the shell initialization code for the given shell.
func ShellInit(shell string) (string, error) {
	switch shell {
	case "fish":
		return fishInit, nil
	case "bash":
		return bashInit, nil
	case "zsh":
		return zshInit, nil
	default:
		return "", fmt.Errorf("unsupported shell: %s (supported: fish, bash, zsh)", shell)
	}
}

const fishInit = `function h
    if test (count $argv) -eq 0
        echo "Usage: h <query>" >&2
        return 1
    end
    set -l dir (hive path $argv)
    and cd $dir
end

function hi
    set -l dir (hive path -i)
    and cd $dir
end

function __hive_projects
    hive list 2>/dev/null | tail -n +3 | awk '{print $2"/"$1; print $1}'
end
complete -c h -f -a '(__hive_projects)'
`

const bashInit = `h() {
    if [ $# -eq 0 ]; then
        echo "Usage: h <query>" >&2
        return 1
    fi
    local dir
    dir="$(hive path "$@")"
    if [ $? -eq 0 ]; then
        cd "$dir" || return 1
    fi
}

hi() {
    local dir
    dir="$(hive path -i)"
    if [ $? -eq 0 ]; then
        cd "$dir" || return 1
    fi
}

_h_completions() {
    local projects
    projects=$(hive list 2>/dev/null | tail -n +3 | awk '{print $2"/"$1; print $1}')
    COMPREPLY=($(compgen -W "$projects" -- "${COMP_WORDS[COMP_CWORD]}"))
}
complete -F _h_completions h
`

const zshInit = `h() {
    if [ $# -eq 0 ]; then
        echo "Usage: h <query>" >&2
        return 1
    fi
    local dir
    dir="$(hive path "$@")"
    if [ $? -eq 0 ]; then
        cd "$dir" || return 1
    fi
}

hi() {
    local dir
    dir="$(hive path -i)"
    if [ $? -eq 0 ]; then
        cd "$dir" || return 1
    fi
}

_h_completions() {
    local -a projects
    projects=("${(@f)$(hive list 2>/dev/null | tail -n +3 | awk '{print $2"/"$1; print $1}')}")
    compadd -- "${projects[@]}"
}
compdef _h_completions h
`
