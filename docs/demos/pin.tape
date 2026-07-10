# Pin a template and scaffold offline
# Requires: spin, git

Output pin.gif
Set Shell "bash"
Set Theme "rose-pine-dawn"
Set FontSize 16
Set Width 1000
Set Height 500

Type "spin add https://github.com/example/go-cli-template.git" Sleep 500ms Enter
Sleep 2s
Type "spin new myapp --template go-cli-template" Sleep 500ms Enter
Sleep 3s
