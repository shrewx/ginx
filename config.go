package ginx

import (
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

var (
	conf = &Configuration{
		Command: &cobra.Command{},
	}
	confFile string
)

type Configuration struct {
	*cobra.Command
}

func Conf(conf interface{}) {
	if confFile == DefaultConfig {
		pwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		confFile = filepath.Join(pwd, confFile)
	}

	if err := cleanenv.ReadConfig(confFile, conf); err != nil {
		panic(err)
	}
}

func AddCommand(cmds ...*cobra.Command) {
	conf.Command.AddCommand(cmds...)
}

func Execute(run func(cmd *cobra.Command, args []string)) {
	conf.Command.Run = run
	conf.Command.Flags().StringVarP(&confFile, "config", "f", "config.yml", "define server conf file path")
	if err := conf.Execute(); err != nil {
		panic(err)
	}
}
