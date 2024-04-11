/*
Copyright © 2024 vincent
*/
package cmd

import (
	"cloudreve_uploader/pkg/cloudreve"
	"cloudreve_uploader/pkg/config"
	"cloudreve_uploader/pkg/utils"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logger = utils.GetLogger()
)

var rootCmd = &cobra.Command{
	Use:   "cloudreve-uploader",
	Short: "upload files to cloudreve",

	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %v", err)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer cancel()

		client, err := cloudreve.NewClient(ctx, config)
		if err != nil {
			return err
		}
		err = client.Login()
		if err != nil {
			return err
		}
		p := viper.Get("path").(string)
		err = client.Upload(args, p)
		if err != nil {
			return err
		}

		links, err := client.DirectLinks(args, p)
		if err != nil {
			return err
		}
		for _, link := range links {
			fmt.Println(link)
		}

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("config", path.Join(config.WorkDir, "config.yaml"), "config file")
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))

	rootCmd.Flags().Bool("direct-link", true, "get direct link")
	_ = viper.BindPFlag("direct-link", rootCmd.Flags().Lookup("direct-link"))

	rootCmd.Flags().String("path", "", "")
	_ = viper.BindPFlag("path", rootCmd.Flags().Lookup("path"))

	cfg := viper.GetViper().Get("config").(string)
	viper.SetConfigName(path.Base(cfg))
	viper.SetConfigType("yaml")
	viper.AddConfigPath(path.Dir(cfg))
	viper.SetEnvPrefix("UPLOADER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	_ = viper.BindEnv("server")
	_ = viper.BindEnv("username")
	_ = viper.BindEnv("password")

	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		logger.Warnf("read config err: %v", err)
	}
}
