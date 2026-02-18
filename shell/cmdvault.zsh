# cmdvault zsh completion and keybindings
# Source this file or add to your .zshrc:
#   source /path/to/cmdvault/shell/cmdvault.zsh

# ============================================================================
# Tab completion
# ============================================================================

_cmdvault() {
    local -a aliases

    # Get aliases from cmdvault
    aliases=($(cmdvault --list-aliases 2>/dev/null))

    # First argument: complete aliases
    if (( CURRENT == 2 )); then
        _describe 'alias' aliases
    else
        # After alias: complete files (for placeholder args)
        _files
    fi
}

compdef _cmdvault cmdvault

# ============================================================================
# Keybinding widget (Ctrl+F to launch picker)
# ============================================================================

cmdvault-widget() {
    zle -I  # Invalidate display
    local result
    result=$(cmdvault --print 2>/dev/null)
    if [[ -n "$result" ]]; then
        LBUFFER+="$result"
    fi
    zle reset-prompt
}

zle -N cmdvault-widget
bindkey '^F' cmdvault-widget
