package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

func signBody(secret, body []byte) []byte {
	computed := hmac.New(sha256.New, secret)
	computed.Write(body)
	return []byte(computed.Sum(nil))
}

func verifySignature(secret []byte, signature string, body []byte) bool {
	const signaturePrefix = "sha256="
	const signatureLength = len(signaturePrefix) + 256/4 // <- number of hex characters needed to represent the 256bit sha sum

	if len(signature) != signatureLength || !strings.HasPrefix(signature, signaturePrefix) {
		return false
	}

	actual := make([]byte, 256/8)                                                           // <- number of bytes needed to represent the 256bit sha sum
	if _, err := hex.Decode(actual, []byte(signature[len(signaturePrefix):])); err != nil { // slice prefix away and decode hex signature
		log.Fatalf("ERROR decoding SHA hex <%s>", err)
		return false
	}
	// Perform constant time comparison to avoid timing attacks
	return hmac.Equal(signBody(secret, body), actual)
}

type HookContext struct {
	Signature string
	Event     string
	Id        string
	Payload   []byte
}

func ParseHook(secret []byte, httpReq *http.Request) (*HookContext, error) {
	hookCtx := HookContext{}

	// sha1 'x-hub-signature' gets only sent for legacy reasons, dont check
	if hookCtx.Signature = httpReq.Header.Get("x-hub-signature-256"); len(hookCtx.Signature) == 0 {
		return nil, errors.New("Header doesnt provide 256bit hash signature")
	}

	if hookCtx.Event = httpReq.Header.Get("x-github-event"); len(hookCtx.Event) == 0 {
		return nil, errors.New("Header doesnt provide event")
	}

	if hookCtx.Id = httpReq.Header.Get("x-github-delivery"); len(hookCtx.Id) == 0 {
		return nil, errors.New("Header doesnt provide delivery id")
	}

	body, err := ioutil.ReadAll(httpReq.Body)

	if err != nil {
		return nil, err
	}

	if !verifySignature(secret, hookCtx.Signature, body) {
		return nil, errors.New("Signature doesnt match secret")
	}

	hookCtx.Payload = body

	return &hookCtx, nil
}
