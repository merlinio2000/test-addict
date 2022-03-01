package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type Config struct {
	DoDebug          bool   `json:"do-debug"`
	GitHubHookSecret string `json:"GitHub-Hook-Secret"` //TODO Refactor to ENV VAR
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

	jsonConfContent, err := ioutil.ReadFile(jsonConfPath)

	if err != nil {
		panic(fmt.Sprintf("Unable to open config file <%s>", err))
	}

	if jsonErr := json.Unmarshal(jsonConfContent, &newConf); jsonErr != nil {
		panic(fmt.Sprintf("Unable to unmarshall config file <%s>", jsonErr))
	}

	return newConf
}
