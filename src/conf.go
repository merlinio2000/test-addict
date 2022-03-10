package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	DoDebug          bool   `json:"do-debug"`
	GitHubHookSecret string // set through env var
	Port             int    `json:"port"`
	Routines         []struct {
		BranchesToProcess []string `json:"branches-to-process"`
		GitCloneDir       string   `json:"git-clone-dir"`
		TestCloneCmdExe   string   `json:"test-clone-cmd-exe"`
		TestCloneCmdArgs  string   `json:"test-clone-cmd-args"`
		RemoveOnSuccess   bool     `json:"remove-on-success"`
		RemoveOnFailure   bool     `json:"remove-on-failure"`
		MailTo            []string `json:"mail-to"`
	} `json:"routines"`
	Mailing struct {
		SMTPHost     string `json:"smtp-host"`
		SMTPPort     string `json:"smtp-port"`
		FromIdentity string `json:"from-identity"`
		FromAddr     string `json:"from-addr"`
		FromPW       string // set through env var
	} `json:"mailing"`
}

func LoadConfig(jsonConfPath string) Config {

	newConf := Config{}

	if newConf.GitHubHookSecret = os.Getenv("GH_HOOK_SECRET"); len(newConf.GitHubHookSecret) == 0 {
		panic("Environment Variable GH_HOOK_SECRET not defined")
	}

	if newConf.Mailing.FromPW = os.Getenv("MAILING_FROM_PW"); len(newConf.Mailing.FromPW) == 0 {
		panic("Environment Variable MAILING_FROM_PW not defined")
	}

	jsonConfContent, err := os.ReadFile(jsonConfPath)
	if err != nil {
		panic(fmt.Sprintf("Unable to open config file <%v>", err))
	}

	if jsonErr := json.Unmarshal(jsonConfContent, &newConf); jsonErr != nil {
		panic(fmt.Sprintf("Unable to unmarshal config file <%+v>", jsonErr))
	}

	return newConf
}
