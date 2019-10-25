package kubeconfig

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	kubeconfigSuffix = ".kubeconfig"
)

type MergedLoader interface {
	GetStartingConfig() (*clientcmdapi.Config, error)
}

type Loader interface {
	GetConfigs() ([]string, []*clientcmdapi.Config, error)

	MergedLoader
}

type ConfigLoader []*clientcmd.ClientConfigLoadingRules

func (l ConfigLoader) GetStartingConfig() (*clientcmdapi.Config, error) {
	if len(l) == 0 {
		return clientcmd.NewDefaultPathOptions().GetStartingConfig()
	}

	return l[0].GetStartingConfig()
}

func (l ConfigLoader) GetConfigs() (files []string, configs []*clientcmdapi.Config, _ error) {
	for _, rule := range l {
		cfg, err := rule.GetStartingConfig()
		if err != nil {
			continue
		}

		files = append(files, rule.ExplicitPath)
		configs = append(configs, cfg)
	}

	return
}

func NewDefaultConfigLoader() (chain ConfigLoader) {
	chain = append(chain, clientcmd.NewDefaultClientConfigLoadingRules())

	fileList, err := ioutil.ReadDir(clientcmd.RecommendedConfigDir)
	if err != nil {
		return []*clientcmd.ClientConfigLoadingRules{}
	}

	for _, f := range deduplicateFiles(fileList) {
		if f.IsDir() {
			continue
		}

		if strings.HasSuffix(f.Name(), kubeconfigSuffix) {
			rule := &clientcmd.ClientConfigLoadingRules{
				ExplicitPath: filepath.Join(clientcmd.RecommendedConfigDir, f.Name()),
			}

			chain = append(chain, rule)
		}
	}

	return chain
}

// getConfigFromFile tries to read a kubeconfig file and if it can't, returns an error.  One exception, missing files result in empty configs, not an error.
func getConfigFromFile(filename string) (*clientcmdapi.Config, error) {
	config, err := clientcmd.LoadFromFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if config == nil {
		config = clientcmdapi.NewConfig()
	}
	return config, nil
}

// deduplicate removes any duplicated values and returns a new slice, keeping the order unchanged
func deduplicate(s []string) []string {
	encountered := map[string]bool{}
	ret := make([]string, 0)
	for i := range s {
		if encountered[s[i]] {
			continue
		}
		encountered[s[i]] = true
		ret = append(ret, s[i])
	}
	return ret
}

// deduplicate removes any duplicated values and returns a new slice, keeping the order unchanged
func deduplicateFiles(files []os.FileInfo) []os.FileInfo {
	encountered := map[string]bool{}
	ret := make([]os.FileInfo, 0)
	for i := range files {
		if encountered[files[i].Name()] {
			continue
		}
		encountered[files[i].Name()] = true
		ret = append(ret, files[i])
	}
	return ret
}
