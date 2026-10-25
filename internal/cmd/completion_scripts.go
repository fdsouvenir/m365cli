package cmd

import "fmt"

func completionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletionScript(), nil
	case "zsh":
		return zshCompletionScript(), nil
	case "fish":
		return fishCompletionScript(), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

func bashCompletionScript() string {
	return `#!/usr/bin/env bash

_m365cli_complete() {
  local IFS=$'\n'
  local completions
  completions=$(m365cli __complete --cword "$COMP_CWORD" -- "${COMP_WORDS[@]}")
  COMPREPLY=()
  if [[ -n "$completions" ]]; then
    COMPREPLY=( $completions )
  fi
}

complete -F _m365cli_complete m365cli
`
}

func zshCompletionScript() string {
	return `#compdef m365cli

autoload -Uz bashcompinit
bashcompinit
` + bashCompletionScript()
}

func fishCompletionScript() string {
	return `function __m365cli_complete
  set -l words (commandline -opc)
  set -l cur (commandline -ct)
  set -l cword (count $words)
  if test -n "$cur"
    set cword (math $cword - 1)
  end
  m365cli __complete --cword $cword -- $words
end

complete -c m365cli -f -a "(__m365cli_complete)"
`
}
