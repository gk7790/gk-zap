package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gk7790/gk-zap/pkg/config"
	m "github.com/gk7790/gk-zap/pkg/config/model"
	"github.com/gk7790/gk-zap/pkg/utils/log"
	"github.com/gk7790/gk-zap/pkg/utils/version"
	"github.com/gk7790/gk-zap/server"
	"github.com/spf13/cobra"
)

var (
	cfgFile          string
	showVersion      bool
	strictConfigMode bool
	serverCfg        m.ServerConfig
)

func init() {
	rootCli.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file of gks")
	rootCli.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "version of gks")
	rootCli.PersistentFlags().BoolVarP(&strictConfigMode, "strict_config", "", true, "strict config parsing mode, unknown fields will cause errors")
	config.RegisterServerConfigFlags(rootCli, &serverCfg)
}

var rootCli = &cobra.Command{
	Use:   "gks",
	Short: "gks is the server of zap (https://github.com/gk7790/gk-zap)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(version.Full())
			return nil
		}
		var (
			svrCfg *m.ServerConfig
			err    error
		)
		if cfgFile != "" {
			svrCfg, err = config.LoadYamlServerConfig(cfgFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else {
			if err := serverCfg.Complete(); err != nil {
				fmt.Printf("failed to complete server config: %v\n", err)
				os.Exit(1)
			}
			svrCfg = &serverCfg
		}

		if err := runServer(svrCfg); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return nil
	},
}

func Execute() {
	if err := rootCli.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServer(cfg *m.ServerConfig) (err error) {
	// 初始化 logger
	log.Init(false, "", log.LevelDebug) // false=不写文件, false=Text格式

	if cfgFile != "" {
		log.Infof("gks uses config file: %s", cfgFile)
	} else {
		log.Infof("gks uses command line arguments for config")
	}

	svr, err := server.NewService(cfg)
	if err != nil {
		return err
	}
	log.Warnf("gks started successfully")

	// 启动服务，并使用一个“永不取消”的根上下文作为运行环境。
	svr.Run(context.Background())
	return nil
}
