# test-addict

A GitHub webhook listener to automatically test upon new commits. Implemented in golang with no dependencies

### Building

(solely requires golang 1.17)
```sh
cd src
go build -o <path-to-the-output-executable>
```


### Usage

```sh
./test-addict -config=/path/to/config/file.json
```

By default, if no Argument is provided the program will search for conf.json in the programs working dir

Optionally, you can specify the ```-file=<path>``` command to load a single request payload from a file instead of listening over HTTP.
This is intended solely for testing & development purposes

My one-liner serve script:
```sh
#!/bin/bash
GH_HOOK_SECRET=":)" MAILING_FROM_PW=":)" ./bin/addict.out >current.log 2>&1
```

### Required OS-Setup

+ Two environment Variables
  - GH_HOOK_SECRET <- The secret you specify when creating the hook on GitHub
  - MAILING_FROM_PW <- The password of the mail address specified in conf.json; Used to send report emails
 
 + git client(reachable by path) has to be present when running this tool.
   Storing git credentials is the users responsibility and will not be altered/setup in any way by this tool.
   (I recommend setting up an authentication token or ssh)
   
 + emailTempl.html has to be present under the programs working dir

### Web

This program will listen&serve under localhost:8081/payload

### Security

To avoid timing attacks and follow best practice this program uses constant time comparison for validation of the SHA256 sum of the received request.
If any request is received that does NOT validate the header/SHA256 checks the program immediately exits, as this means theres probably someone attacking.

*TODO*: deeper look into the programs security

### Exposing from localhost to the WWW

For dev-purposes i personally use ngrok to expose my localhost to the internet to receive the hook payload from github.
**Where ever possible this should be done in an isolated network and with a computer you dont use for anything else.
Even though ngrok seems secure(for now) and test-addict verifies the request with scrutiny you are still technically opening a hole through your network wide open**


### Finding a mail provider

Most free mail providers will quickly recognize if you are sending automated traffic through SMTP and block your account.
For this reason I am using sendgrid. It is quick and easy to setup (exactly for this purpose) and gives you a free daily quota of a maximum of 100 mails.

