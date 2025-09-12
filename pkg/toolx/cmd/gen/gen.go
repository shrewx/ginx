package gen

import "github.com/spf13/cobra"

var CmdGen = &cobra.Command{
	Use:   "gen",
	Short: "generate file command",
}

func init() {
	CmdGen.AddCommand(statusErrorCommand())
	CmdGen.AddCommand(statusErrorYamlCommand())
	CmdGen.AddCommand(openapiCommand())
	CmdGen.AddCommand(clientCommand())
	CmdGen.AddCommand(enumCommand())
	CmdGen.AddCommand(i18nCommand())
	CmdGen.AddCommand(statusI18nYamlCommand())
}
