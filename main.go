package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	color "github.com/logrusorgru/aurora"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/alanchchen/kluster/pkg/kubeconfig"
)

const (
	appName       = "kluster"
	currentSymbol = "⚡"
)

var (
	cfg          *viper.Viper = viper.New()
	cfgFile      string
	configLoader = kubeconfig.NewDefaultConfigLoader()
	ioStreams    = genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
)

func init() {
	mainCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is $HOME/.%s/config.yaml)", appName))

	// kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	// matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	// mainCmd.AddCommand(cmdconfig.NewCmdConfig(f, , ioStreams))

}

var mainCmd = cobra.Command{
	Use:   appName,
	Short: appName + ` is a utility to manage and switch between kubectl config files`,
	Long: appName + ` is a utility to manage and switch between kubectl config files.
		%s                  : list the clusters
        %s <NAME>           : switch to cluster <NAME>
        %s -                : switch to the previous cluster
	`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		var cfgFile string
		if cfgFlag := cmd.Flags().Lookup("config"); cfgFlag != nil {
			cfgFile = cfgFlag.Value.String()
		}

		if cfgFile != "" { // enable ability to specify config file via flag
			cfg.SetConfigFile(cfgFile)
		} else {
			cfg.SetConfigName("config")
			cfg.SetConfigType("yaml")
			cfg.AddConfigPath("$HOME/." + appName)
		}

		if err = cfg.BindPFlags(cmd.Flags()); err != nil {
			// either nil or a wrapped error
			return err
		}

		cfg.AutomaticEnv() // read in environment variables that match
		cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

		// If a config file is found, read it in.
		// Ignore the error if config file is not found
		if err = cfg.ReadInConfig(); err == nil {
			cmd.Println("Using config file:", cfg.ConfigFileUsed())
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runGetClusters(ioStreams.Out, configLoader)
		} else if len(args) == 1 {
			return setCluster(os.Args[0])
		}

		return nil
	},
}

func listClusters() error {
	home, _ := os.UserHomeDir()
	kubeDir := filepath.Join(home, ".kube")
	files, err := ioutil.ReadDir(kubeDir)
	if err != nil {
		return err
	}

	currentCluster := klusterName(os.Getenv("KUBECONFIG"))

	for _, f := range files {
		if f.IsDir() {
			st, err := os.Stat(filepath.Join(kubeDir, f.Name(), "kubeconfig"))
			if err != nil {
				continue
			}

			if !st.IsDir() {
				cluster := filepath.Base(f.Name())

				if cluster == currentCluster {
					fmt.Println(color.Yellow(cluster), currentSymbol)
				} else {
					fmt.Println(cluster)
				}
			}
		}
	}

	return nil
}

func runGetClusters(out io.Writer, loader kubeconfig.Loader) error {
	config, err := loader.GetStartingConfig()
	if err != nil {
		return err
	}

	currentCtx, _ := config.Contexts[config.CurrentContext]

	files, configs, err := loader.GetConfigs()
	if err != nil {
		return err
	}

	w := tablewriter.NewWriter(out)
	w.SetHeader([]string{"NAME", "CONFIG", "SERVER"})
	w.SetBorder(false) // Set Border to false

	clusterNameConfigFiles := map[string]string{}
	clusterNameCluster := map[string]*clientcmdapi.Cluster{}
	clusterNames := make([]string, 0, len(config.Clusters))
	for i, cfg := range configs {
		for name, cluster := range cfg.Clusters {
			clusterNames = append(clusterNames, name)
			clusterNameConfigFiles[name] = filepath.Base(files[i])
			clusterNameCluster[name] = cluster
		}
	}

	sort.Strings(clusterNames)

	for _, name := range clusterNames {
		cluster := clusterNameCluster[name]

		info := make([]string, 3)
		if currentCtx != nil && currentCtx.Cluster == name {
			info[0] = currentSymbol + color.Yellow(name).String()
		} else {
			info[0] = "  " + name
		}
		info[1] = clusterNameConfigFiles[name]
		info[2] = cluster.Server

		w.Append(info)
	}

	w.Render()

	return nil
}

func setCluster(name string) error {
	return listClusters()
}

func kubeconfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube")
}

func klusterName(kubeconfig string) string {
	if b := filepath.Base(kubeconfig); b == "kubeconfig" {
		return filepath.Base(filepath.Dir(kubeconfig))
	}

	return ""
}

func main() {
	if err := mainCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
