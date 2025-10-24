package config

import (
	m "github.com/gk7790/gk-zap/pkg/config/model"
	"github.com/spf13/cobra"
)

func RegisterServerConfigFlags(cmd *cobra.Command, c *m.ServerConfig) {
	cmd.PersistentFlags().StringVarP(&c.BindAddr, "bind_addr", "", "0.0.0.0", "bind address")
	cmd.PersistentFlags().IntVarP(&c.BindPort, "bind_port", "p", 7000, "bind port")

}
