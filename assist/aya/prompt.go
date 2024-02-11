package main

import (
	_ "embed"
	"os"
	"runtime"
	"strings"
	"time"
)

//go:embed prompt1.txt
var promptTemplate1 string

//go:embed prompt2.txt
var promptTemplate2 string

func genPrompt() string {
	date := time.Now().Format(time.DateOnly)
	cwd, _ := os.Getwd()
	isGit := "is NOT"
	if _, err := os.Stat(".git"); !os.IsNotExist(err) {
		isGit = "IS"
	}

	prompt := promptTemplate1
	prompt = strings.Replace(prompt, "{date}", date, 1)
	prompt = strings.Replace(prompt, "{cwd}", cwd, 1)
	prompt = strings.Replace(prompt, "{is_git}", isGit, 1)
	prompt = strings.Replace(prompt, "{os}", runtime.GOOS, 1)
	return prompt
}
