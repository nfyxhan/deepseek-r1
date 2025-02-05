/*
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "deepseek",
	Short: "nfyxhan's command tools",
	Long: `nfyxhan's command tools
`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.nfyxhan.yaml)")

}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".nfyxhan")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		_ = fmt.Sprintf("Using config file: %s", viper.ConfigFileUsed())
	}
}

func GetConfig(key string, obj interface{}) error {
	config := viper.Get(key)
	d1, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	configData, err := json.Marshal(config)
	if err != nil {
		return err
	}
	m1 := make(map[string]interface{})
	configMap := make(map[string]interface{})
	if err := json.Unmarshal(d1, &m1); err != nil {
		return err
	}
	if err := json.Unmarshal(configData, &configMap); err != nil {
		return err
	}
	m3 := MergeMaps(m1, configMap, false)
	data, err := json.Marshal(m3)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	return nil
}

func MergeMaps(defaultMap, customMap map[string]interface{}, coverOnly bool) map[string]interface{} {
	out := make(map[string]interface{}, len(defaultMap))
	for k, v := range defaultMap {
		out[k] = v
	}
	for k, v := range customMap {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = MergeMaps(bv, v, coverOnly)
					continue
				}
			}
		}
		if _, ok := out[k]; ok || !coverOnly {
			out[k] = v
		}
	}
	return out
}
