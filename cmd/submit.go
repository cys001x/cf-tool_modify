package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gitcode.com/sheng_wang/cf-tool_modify/client"
	"gitcode.com/sheng_wang/cf-tool_modify/config"
	"github.com/fatih/color"
)

// Submit command
func Submit() (err error) {
	cln := client.Instance
	cfg := config.Instance
	info := Args.Info
	filename, index, err := getOneCode(Args.File, "", cfg.Template)
	if err != nil {
		return
	}

	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	fmt.Printf("submit file %s\n", filename)
	source := string(bytes)

	lang := cfg.Template[index].Lang
	if err = cln.Submit(info, lang, source); err != nil {
		if err = loginAgain(cln, err); err == nil {
			err = cln.Submit(info, lang, source)
		}
	}
	if err != nil {
		return
	}
	printFailedSubmissionTests(cln)
	return
}

// printFailedSubmissionTests pulls the judge details of the submission that
// just finished and prints the test cases it failed.
func printFailedSubmissionTests(cln *client.Client) {
	if cln.LastVerdict == "" || cln.LastVerdict == "OK" || cln.LastSubmission == nil {
		return
	}

	detail, err := cln.SubmissionDetail(*cln.LastSubmission)
	if err != nil {
		color.Yellow("Cannot fetch test details: %v", err)
		return
	}
	if detail.TestsError != nil {
		color.Yellow("Test details unavailable: %v", detail.TestsError)
		return
	}
	if detail.CompilationError {
		fmt.Printf("\n")
		printCompilationError(detail)
		return
	}
	printFailedTests(detail)

	dir, wdErr := os.Getwd()
	if wdErr != nil {
		return
	}
	saved, saveErr := saveFailedTestFiles(detail, dir)
	if saveErr != nil {
		color.Yellow("Cannot save test files: %v", saveErr)
		return
	}
	if len(saved) > 0 {
		color.Green("Saved failed tests: %v", strings.Join(saved, ", "))
	}
}
