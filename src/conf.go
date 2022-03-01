package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type Config struct {
	DoDebug          bool   `json:"do-debug"`
	GitHubHookSecret string
	Port             int    `json:"port"`
	Routines         []struct {
		BranchesToProcess []string `json:"branches-to-process"`
		GitCloneDir       string   `json:"git-clone-dir"`
		CleanupCloneCmd   string   `json:"cleanup-clone-cmd"`
		TestCloneCmd      string   `json:"test-clone-cmd"`
	} `json:"routines"`
}

func LoadConfig(jsonConfPath string) Config {

	newConf := Config{}
	
	if newConf.GitHubHookSecret = os.Getenv("GH_HOOK_SECRET"); len(newConf.GitHubHookSecret) == 0 {
		panic("Environment Variable GH_HOOK_SECRET not defined")
	}

	jsonConfContent, err := ioutil.ReadFile(jsonConfPath)

	if err != nil {
		panic(fmt.Sprintf("Unable to open config file <%s>", err))
	}

	if jsonErr := json.Unmarshal(jsonConfContent, &newConf); jsonErr != nil {
		panic(fmt.Sprintf("Unable to unmarshal config file <%s>", jsonErr))
	}

	return newConf
}
