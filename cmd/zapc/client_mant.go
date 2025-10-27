package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gk7790/gk-zap/client"
	"github.com/gk7790/gk-zap/pkg/utils/version"
	"github.com/spf13/cobra"
)

var (
	cfgFile          string
	cfgDir           string
	showVersion      bool
	strictConfigMode bool
)

func init() {
	rootCli.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./frpc.ini", "config file of frpc")
	rootCli.PersistentFlags().StringVarP(&cfgDir, "config_dir", "", "", "config directory, run one frpc service for each file in config directory")
	rootCli.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "version of frpc")
	rootCli.PersistentFlags().BoolVarP(&strictConfigMode, "strict_config", "", true, "strict config parsing mode, unknown fields will cause an errors")
}

var rootCli = &cobra.Command{
	Use:   "gkc",
	Short: "gkc is the server of zap (https://github.com/gk7790/gk-zap)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(version.Full())
			return nil
		}
		// Do not show command usage here.
		err := runClient(cfgFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return nil
		return nil
	},
}

func Execute() {
	if err := rootCli.Execute(); err != nil {
		os.Exit(1)
	}
}

func runClient(cfgFilePath string) error {
	svr, err := client.NewService(client.ServiceOptions{})
	if err != nil {
		return err
	}
	return svr.Run(context.Background())
}
