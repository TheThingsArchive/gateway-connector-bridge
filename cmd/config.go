// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/spf13/viper"
)

// EnvPrefix is the environment prefix that is used for configuration
const EnvPrefix = "bridge"

var cfgFile string

func initConfig() {
	viper.SetEnvPrefix(EnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		err := viper.ReadInConfig()
		if err != nil {
			fmt.Println("Error when reading config file:", err)
		} else if err == nil {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
	viper.BindEnv("debug")

	defaultID := "unknown"
	if user, err := user.Current(); err == nil {
		defaultID = user.Username
	}
	if hostname, err := os.Hostname(); err == nil {
		defaultID += "@" + hostname
	}
	viper.SetDefault("id", defaultID)
}

var config = viper.GetViper()
