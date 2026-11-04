package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"gitcode.com/sheng_wang/cf-tool_modify/client"
	"gitcode.com/sheng_wang/cf-tool_modify/config"
	"github.com/fatih/color"
	ansi "github.com/k0kubun/go-ansi"
	"github.com/olekukonko/tablewriter"
)

// maxDetailLines limits how much of a test input/output/answer is printed.
const maxDetailLines = 40

func formatSize(size uint64) string {
	if size > 1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(size)/1024.0/1024.0)
	} else if size > 1024 {
		return fmt.Sprintf("%.2f KB", float64(size)/1024.0)
	}
	return fmt.Sprintf("%v B", size)
}

func truncateLines(text string, max int) string {
	text = strings.TrimRight(text, "\r\n")
	if text == "" {
		return "(empty)"
	}
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	if len(lines) <= max {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:max], "\n") + fmt.Sprintf("\n... (%v more lines)", len(lines)-max)
}

func printIndented(text string) {
	for _, line := range strings.Split(text, "\n") {
		fmt.Printf("      %v\n", line)
	}
}

func printField(name, value string) {
	if value == "" {
		return
	}
	fmt.Printf("  %-8v: %v\n", name, value)
}

// Detail command prints the summary and the per-test results of a submission.
func Detail() (err error) {
	cln := client.Instance
	info := Args.Info
	if info.SubmissionID == "" && cln.LastSubmission != nil {
		info = *cln.LastSubmission
	}

	detail, err := cln.SubmissionDetail(info)
	if err != nil {
		return
	}

	fmt.Printf("Submission %v\n", detail.ID)
	printField("contest", detail.ContestName)
	printField("problem", detail.ProblemName)
	printField("author", detail.Author)
	printField("lang", detail.Lang)

	verdict := detail.Verdict
	if strings.Contains(strings.ToLower(verdict), "accepted") {
		verdict = color.GreenString(verdict)
	} else if verdict != "" {
		verdict = color.RedString(verdict)
	}
	printField("verdict", verdict)
	if detail.Time > 0 {
		printField("time", fmt.Sprintf("%v ms", detail.Time))
	}
	if detail.Memory > 0 {
		printField("memory", formatSize(detail.Memory))
	}
	printField("sent", detail.Sent)
	printField("judged", detail.Judged)
	if link, linkErr := info.SubmissionURL(config.Instance.Host); linkErr == nil {
		printField("link", link)
	}

	if detail.TestsError != nil {
		fmt.Printf("\n")
		color.Yellow("Test details unavailable: %v", detail.TestsError)
		return
	}

	if detail.CompilationError {
		fmt.Printf("\n")
		color.Red("Compilation error")
		for _, test := range detail.Tests {
			if test.Checker != "" {
				printIndented(truncateLines(test.Checker, maxDetailLines))
			}
		}
		return
	}

	if len(detail.Tests) == 0 {
		return
	}

	var buf bytes.Buffer
	table := tablewriter.NewWriter(io.Writer(&buf))
	table.SetHeader([]string{"#", "verdict", "time", "memory", "exit", "checker"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetCenterSeparator("|")
	table.SetAutoWrapText(false)
	for _, test := range detail.Tests {
		checker := strings.TrimSpace(test.Checker)
		table.Append([]string{
			fmt.Sprintf("%v", test.Index),
			test.Verdict,
			fmt.Sprintf("%v ms", test.Time),
			formatSize(test.Memory),
			test.ExitCode,
			checker,
		})
	}
	table.Render()

	fmt.Printf("\nTests: %v\n", len(detail.Tests))
	scanner := bufio.NewScanner(io.Reader(&buf))
	for scanner.Scan() {
		ansi.Println(scanner.Text())
	}

	printed := false
	for _, test := range detail.Tests {
		if test.Passed {
			continue
		}
		printed = true
		fmt.Printf("\nTest %v (%v)\n", test.Index, test.Verdict)
		if strings.TrimSpace(test.Input) != "" {
			fmt.Printf("  input:\n")
			printIndented(truncateLines(test.Input, maxDetailLines))
		}
		if strings.TrimSpace(test.Output) != "" {
			fmt.Printf("  output:\n")
			printIndented(truncateLines(test.Output, maxDetailLines))
		}
		if strings.TrimSpace(test.Answer) != "" {
			fmt.Printf("  expected:\n")
			printIndented(truncateLines(test.Answer, maxDetailLines))
		}
		if strings.TrimSpace(test.Checker) != "" {
			fmt.Printf("  checker:\n")
			printIndented(truncateLines(test.Checker, maxDetailLines))
		}
		if strings.TrimSpace(test.Diagnostics) != "" {
			fmt.Printf("  diagnostics:\n")
			printIndented(truncateLines(test.Diagnostics, maxDetailLines))
		}
	}
	if !printed && detail.Verdict != "" {
		fmt.Printf("\nAll %v tests passed.\n", len(detail.Tests))
	}
	return
}
