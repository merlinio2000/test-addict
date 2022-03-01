// https://docs.github.com/en/developers/webhooks-and-events/webhooks/creating-webhooks
// Created by Merlin Maggi, maggimer@students.zhaw.ch
// Fair use as long as credit is given

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

var config Config

// Used to access GitHub JSON Payload from the program
// go JSON Unmarshal ignores case of members
type ghPayload struct {
	// TODO get from JSON body
	HeaderDeliveryId string
	// which branch was pushed to
	Ref string
	// commit id before push
	Before string
	// commit id after push
	After string

	CloneURL     string `json:"clone_url"`
	MasterBranch string `json:"master_branch"`
	Pusher       struct {
		name  string
		email string
	}
	// url for quick compare in the browser
	CompareURL string `json:"compare"`
}

func RunRoutine(payload *ghPayload) {
	// find the correct routine
	splitRefPath := strings.Split(payload.Ref, "/")
	branch := splitRefPath[len(splitRefPath)]

	for _, routine := range config.Routines {
		for _, branchToDo := range routine.BranchesToProcess {
			if branchToDo == branch {

				gitCloneDir := routine.GitCloneDir + payload.HeaderDeliveryId

				if cloneDirErr := os.Mkdir(gitCloneDir, os.ModePerm); cloneDirErr != nil {
					log.Printf("Error creating clone directory(<%s>), wont process this payload <%s>\n", payload.HeaderDeliveryId, cloneDirErr)
					return
				}
				// Use git to clone the branch that was pushed to
				gitCloneCmd := exec.Command("git", "clone", "--branch "+branch, payload.CloneURL)
				// Execute command from specified directory
				gitCloneCmd.Dir = gitCloneDir
				// stderr gets merged to stdout, runs command
				stdout, gitErr := gitCloneCmd.CombinedOutput()
				if gitErr != nil {
					log.Printf("Error cloning from git <%s>, skipping this one\n", gitErr)
					return
				}

				if config.DoDebug {
					log.Println("DEBUG stdout of git")
					log.Println(stdout)
				}

				testCmd := exec.Command(routine.TestCloneCmdExe, routine.TestCloneCmdArgs)
				testCmd.Dir = gitCloneDir
				testoutput, testErr := testCmd.CombinedOutput()
				if testErr != nil {
					log.Printf("ERROR running Test on branch <%s> by %s (%s)\n", branch, payload.Pusher.name, payload.Pusher.email)
					log.Printf("ERROR description <%s>\n", testErr)
					log.Println("Command output:")
					log.Println(testoutput)
					// TODO SendMail(...)
				} else {
					log.Printf("Test Successfull on branch <%s> by %s (%s)\n", branch, payload.Pusher.name, payload.Pusher.email)
					if config.DoDebug {
						log.Println("Command output:")
						log.Println(testoutput)
					}
					// remove clone after successfull test
					defer os.RemoveAll(gitCloneDir)
				}

			}
		}
	}
}

// Called by the server to handle the '/payload' route
func HookHandler(httpResp http.ResponseWriter, httpReq *http.Request) {

	var HandleFail = func(reqStatus int, errMsg string) {
		httpResp.WriteHeader(reqStatus)
		io.WriteString(httpResp, "{}")
		log.Fatalln(errMsg)
	}

	if config.DoDebug {
		for name, headers := range httpReq.Header {
			for _, h := range headers {
				fmt.Printf("HEADER> %s: %s\n", name, h)
			}
		}
	}

	hookCtx, err := ParseHook([]byte(config.GitHubHookSecret), httpReq)

	httpResp.Header().Set("Content-type", "application/json")

	if err != nil {
		HandleFail(http.StatusBadRequest, fmt.Sprintf("ERROR processing hook <%s>\n", err))
		return
	}

	log.Printf("Received event <%s>", hookCtx.Event)

	if config.DoDebug {
		log.Printf("Payload <\n%s\n>\n", hookCtx.Payload)
	}

	// parse `hookCtx.Payload` or do additional processing here
	if hookCtx.Event != "push" {
		log.Printf("Wont process event type<%s>", hookCtx.Event)
		httpResp.WriteHeader(http.StatusNotImplemented)
	} else {
		var recvPayload ghPayload
		if jsonErr := json.Unmarshal(hookCtx.Payload, &recvPayload); jsonErr == nil {
			RunRoutine(&recvPayload)
			httpResp.WriteHeader(http.StatusOK)
		} else { // Error in JSON Request body from GitHub
			HandleFail(http.StatusBadRequest, fmt.Sprintf("ERROR unmarshalling JSON request Payload <%s>", err))
			return
		}
	}
	io.WriteString(httpResp, "{}")
	return
}

// Program entry point
func main() {
	//by default look for the config file in the directory the program was executed, if its a symlink it might be invalid by now
	exe, exeErr := os.Executable()
	if exeErr != nil {
		panic("Unable to determine executable directory")
	}
	configArg := flag.String("config", exe, "path to the JSON config file")
	flag.Parse()
	config = LoadConfig(*configArg)

	http.HandleFunc("/payload", HookHandler)

	http.ListenAndServe(":8081", nil)
}
