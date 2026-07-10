# Create a template with spin init and test it
# Requires: spin

Output template-init.gif
Set Shell "bash"
Set Theme "rose-pine-dawn"
Set FontSize 16
Set Width 1000
Set Height 500

Type "spin init my-cli-template" Sleep 500ms Enter
Sleep 1s
Type "spin new myapp --template ./my-cli-template --dry-run" Sleep 500ms Enter
Sleep 3s
