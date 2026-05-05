module github.com/dgrieser/nickpit

go 1.25.0

require (
	github.com/sashabaranov/go-openai v1.41.2
	github.com/spf13/cobra v1.8.1
	golang.org/x/term v0.42.0
	golang.org/x/tools v0.44.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
)

replace github.com/sashabaranov/go-openai => github.com/dgrieser/go-openai v0.0.0-20260417101125-737ccbf46cee
