// https://docs.github.com/en/developers/webhooks-and-events/webhooks/creating-webhooks
// Created by Merlin Maggi, maggimer@students.zhaw.ch
// Fair use as long as credit is given

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"strings"
)

// Global config Variable, gets initialized in main()
var config Config

// Used to access GitHub JSON Payload from the program
// go JSON Unmarshal ignores case of members
type ghPayload struct {
	// which branch was pushed to
	Ref string
	// commit id before push
	Before string
	// commit id after push
	After      string
	Repository struct {
		CloneURL     string `json:"clone_url"`
		MasterBranch string `json:"master_branch"`
	} `json:"repository"`
	Pusher struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
	// url for quick compare in the browser
	CompareURL string `json:"compare"`
}

// Used to contain information relevant to an email
type MailReport struct {
	Payload *ghPayload
	Success bool
	Message string
}

// golang smtp AUTH LOGIN fix; https://github.com/go-gomail/gomail/issues/16
type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("Unkown fromServer:" + string(fromServer))
		}
	}
	return nil, nil
}

// Send Test Execution Report to recipients
// ${pusher} inside of recipients will be replaced with payload.Pusher.email
func SendReportMail(report MailReport, recipients []string) {

	payload := report.Payload

	if config.DoDebug {
		fmt.Printf("Report: payload: %+v\n", *payload)
	}

	// Fill the email body template
	templ, templErr := template.ParseFiles("emailTempl.html")

	if templErr != nil {
		log.Printf("Error parsing html email template <%v>, wont send mail", templErr)
		return
	}

	subject := "Test %s on " + payload.Ref
	if report.Success {
		subject = fmt.Sprintf(subject, "successfull")
	} else {
		subject = fmt.Sprintf(subject, "FAILED")
	}

	var mailBody bytes.Buffer

	header := make(map[string]string)
	header["From"] = config.Mailing.FromAddr
	header["To"] = strings.Join(recipients, ",")
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=\"utf-8\""

	for k, v := range header {
		mailBody.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	// newline before mail body
	mailBody.WriteString("\r\n")

	if templFillErr := templ.Execute(&mailBody, report); templFillErr != nil {
		log.Printf("Couldnt fill email body template <%v>", templFillErr)
		log.Println("Defaulting back to %+v representation; Some data might have still been written and the result may appear scuffed")
		mailBody.WriteString(fmt.Sprintf("%+v", *payload))
	}

	// replace placeholders in recipients
	for idx, recp := range recipients {
		if recp == "${pusher}" {
			recipients[idx] = payload.Pusher.Email
		}
	}

	log.Printf("Sending mail to %s", strings.Join(recipients, ","))

	// Instatiate overridden SMTP Auth interface
	smtpAuth := LoginAuth(config.Mailing.FromIdentity, config.Mailing.FromPW)

	mailErr := smtp.SendMail(config.Mailing.SMTPHost+":"+config.Mailing.SMTPPort, smtpAuth, config.Mailing.FromAddr, recipients, mailBody.Bytes())

	if mailErr != nil {
		log.Printf("Error sending report mail <%v>", mailErr)
	}

}

// Actual Logic of the Program
// Clones the repo (only branch that was pushed to) living at payload.CloneURL into config.Routines[i].GitCloneDir
// it does this for EVERY element in config.Routines where the branch is included in routine.BranchesToProcess
func RunRoutine(payload *ghPayload, deliveryID string) {

	if config.DoDebug {
		log.Printf("Processing delivery %s with the following payload\n", deliveryID)
		log.Printf("%+v", *payload)
	}
	const illegalChars = " |&$"
	// find the correct routine
	splitRefPath := strings.Split(payload.Ref, "/")
	branch := splitRefPath[len(splitRefPath)-1] // last element should be only the branch name

	if strings.ContainsAny(branch, illegalChars) {
		log.Fatalf("Discovered Illegal Chars that could be used for manipulation for the OS command in branchname<%s>\n", branch)
	}

	if strings.ContainsAny(payload.Repository.CloneURL, illegalChars) {
		log.Fatalf("Discovered Illegal Chars that could be used for manipulation for the OS command in repository url<%s>\n", payload.Repository.CloneURL)
	}

	for _, routine := range config.Routines {
		for _, branchToDo := range routine.BranchesToProcess {
			if branchToDo == branch {
				// guaranteed to be unique as long as github works correctly (and the correct argument was passed :) )
				gitCloneDir := routine.GitCloneDir + deliveryID + branch

				log.Printf("Will use '%s' as git directory", gitCloneDir)

				if cloneDirErr := os.Mkdir(gitCloneDir, os.ModePerm); cloneDirErr != nil {
					log.Printf("Error creating clone directory(<%s>), wont process this payload <%v>\n", gitCloneDir, cloneDirErr)
					return
				}
				// Use git to clone the branch that was pushed to
				// . will clone into the current directory (no sub directory)
				gitCloneCmd := exec.Command("git", "clone", "--branch", branch, payload.Repository.CloneURL, ".")
				// Execute command from specified directory
				gitCloneCmd.Dir = gitCloneDir

				if config.DoDebug {
					log.Printf("Will be using <%s> as git command", gitCloneCmd.String())
				}

				// stderr gets merged to stdout, runs command
				gitStdout, gitErr := gitCloneCmd.CombinedOutput()
				if gitErr != nil {
					log.Printf("Error cloning from git <%v>, skipping this one\n", gitErr)
					log.Println(string(gitStdout[:]))
					return
				}

				if config.DoDebug {
					log.Println("DEBUG stdout of git")
					log.Println(string(gitStdout[:]))
				}

				// Run tests on the cloned repo
				testCmd := exec.Command(routine.TestCloneCmdExe, routine.TestCloneCmdArgs)
				testCmd.Dir = gitCloneDir
				testoutput, testErr := testCmd.CombinedOutput()

				var success bool
				var mailMsg string

				if testErr != nil {
					log.Printf("ERROR running Test on branch <%s> by %s (%s)\n", branch, payload.Pusher.Name, payload.Pusher.Email)
					log.Printf("ERROR description <%v>\n", testErr)
					log.Println("Command output:")
					log.Println(string(testoutput[:]))

					success = false
					mailMsg = fmt.Sprintf("%+v\n%s\n", testErr, string(testoutput[:]))

					if routine.RemoveOnFailure {
						// remove clone after successfull test
						defer os.RemoveAll(gitCloneDir)
					}
				} else {
					log.Printf("Test Successfull on branch <%s> by %s (%s)\n", branch, payload.Pusher.Name, payload.Pusher.Email)

					if config.DoDebug {
						log.Println("Command output:")
						log.Println(string(testoutput[:]))
					}

					success = true
					mailMsg = fmt.Sprintf("%s\n", string(testoutput[:]))

					if routine.RemoveOnSuccess {
						// remove clone after successfull test
						defer os.RemoveAll(gitCloneDir)
					}
				}
				SendReportMail(MailReport{Payload: payload, Success: success, Message: mailMsg}, routine.MailTo)

			}
		}
	}
}

// Called by the server to handle the '/payload' route
func HookHandler(httpResp http.ResponseWriter, httpReq *http.Request) {

	var HandleFail = func(reqStatus int, errMsg string) {
		httpResp.WriteHeader(reqStatus)
		io.WriteString(httpResp, "{}")
		log.Fatalln(errMsg) // Fatal will quit the program, following returns are simply for readability
	}

	if config.DoDebug {
		for name, headers := range httpReq.Header {
			for _, h := range headers {
				fmt.Printf("HEADER> %s: %s\n", name, h)
			}
		}
	}

	httpResp.Header().Set("Content-type", "application/json")

	hookCtx, hookErr := ParseHook([]byte(config.GitHubHookSecret), httpReq)

	if hookErr != nil {
		HandleFail(http.StatusBadRequest, fmt.Sprintf("ERROR processing hook <%v>\n", hookErr))
		return
	}

	log.Printf("Received event <%s>\n", hookCtx.Event)

	if config.DoDebug {
		log.Printf("Payload <\n%s\n>\n", hookCtx.Payload)
	}

	// payload parsing
	if hookCtx.Event != "push" {
		log.Printf("Wont process event type<%s>", hookCtx.Event)
		httpResp.WriteHeader(http.StatusNotImplemented)
	} else {
		var recvPayload ghPayload
		if jsonErr := json.Unmarshal(hookCtx.Payload, &recvPayload); jsonErr == nil {

			log.Printf("Will be processing delivery %s", hookCtx.Id)
			// Actual programm Logic
			RunRoutine(&recvPayload, hookCtx.Id)

			log.Printf("All good, bye")
			httpResp.WriteHeader(http.StatusOK)
		} else { // Error in JSON Request body from GitHub
			HandleFail(http.StatusBadRequest, fmt.Sprintf("ERROR unmarshalling JSON request Payload <%v>", jsonErr))
			return
		}
	}
	io.WriteString(httpResp, "{}")
}

// Program entry point
func main() {
	//by default look for the config file in the directory the program was executed, if its a symlink it might be invalid by now
	exe, exeErr := os.Getwd()
	if exeErr != nil {
		panic(fmt.Sprintf("Unable to determine executable directory <%v>", exeErr))
	}
	exe += "/conf.json"
	log.Printf("Picking up default config at %s\n", exe)
	configArg := flag.String("config", exe, "path to the JSON config file")
	loadFileArg := flag.String("file", "", "load Payload from file instead of listening")
	flag.Parse() // automatically handles -help, unhandled flags, etc...

	config = LoadConfig(*configArg)
	if *loadFileArg != "" {
		payloadFile, payloadErr := os.ReadFile(*loadFileArg)
		if payloadErr != nil {
			panic(fmt.Sprintf("Unable to open payload file <%v>", payloadErr))
		}

		loadedPayload := &ghPayload{}
		if jsonErr := json.Unmarshal(payloadFile, loadedPayload); jsonErr != nil {
			panic(fmt.Sprintf("Unable to unmarshal payload file <%+v>", jsonErr))
		}

		RunRoutine(loadedPayload, "loaded-from-file")
	} else {
		// spawns a new go routine for each request
		// access to 'config' still IS thread safe as it wont be modified after loading
		http.HandleFunc("/payload", HookHandler)

		http.ListenAndServe(":8081", nil)
	}
}
