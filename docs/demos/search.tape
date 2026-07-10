# Search a registry and add a result
# Requires: spin, git

Output search.gif
Set Shell "bash"
Set Theme "rose-pine-dawn"
Set FontSize 16
Set Width 1000
Set Height 500

Type "spin search go cli" Sleep 500ms Enter
Sleep 2s
Type "spin add official/go-cli" Sleep 500ms Enter
Sleep 2s
