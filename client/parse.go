package client

import (
	"fmt"
	"html"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gitcode.com/sheng_wang/cf-tool_modify/util"

	"github.com/k0kubun/go-ansi"

	"github.com/fatih/color"
)

func findSample(body []byte) (input [][]byte, output [][]byte, err error) {
	irg := regexp.MustCompile(`class="input"[\s\S]*?<pre([^>]*)>([\s\S]*?)</pre>`)
	org := regexp.MustCompile(`class="output"[\s\S]*?<pre([^>]*)>([\s\S]*?)</pre>`)
	a := irg.FindAllSubmatch(body, -1)
	b := org.FindAllSubmatch(body, -1)
	if a == nil || b == nil || len(a) != len(b) {
		return nil, nil, fmt.Errorf("Cannot parse sample with input %v and output %v", len(a), len(b))
	}
	anyTag := regexp.MustCompile(`<[^>]*>`)
	multiNL := regexp.MustCompile(`\n+`)
	dataContent := regexp.MustCompile(`data-content="([\s\S]*?)"`)
	filter := func(attrs, src []byte) []byte {
		// Codeforces wraps every sample line in <div class="test-example-line">
		// and keeps the exact sample text in the pre tag's data-content
		// attribute. Prefer it: stripping tags would collapse blank lines that
		// are part of the sample.
		var s string
		if m := dataContent.FindSubmatch(attrs); m != nil {
			s = html.UnescapeString(string(m[1]))
		} else {
			src = anyTag.ReplaceAll(src, []byte("\n"))
			s = html.UnescapeString(string(src))
			s = multiNL.ReplaceAllString(s, "\n")
		}
		return []byte(strings.TrimSpace(s) + "\n")
	}
	for i := 0; i < len(a); i++ {
		input = append(input, filter(a[i][1], a[i][2]))
		output = append(output, filter(b[i][1], b[i][2]))
	}
	return
}

// ParseProblem parse problem to path. mu can be nil
func (c *Client) ParseProblem(URL, path string, mu *sync.Mutex) (samples int, standardIO bool, err error) {
	body, err := util.GetBody(c.client, URL)
	if err != nil {
		return
	}

	_, err = findHandle(body)
	if err != nil {
		return
	}

	input, output, err := findSample(body)
	if err != nil {
		return
	}

	standardIO = true
	// The class attribute gained an extra modifier ("input-file input-standard"),
	// so match the prefix instead of the exact old attribute value.
	stdIO := regexp.MustCompile(`class="input-file[^"]*"[\s\S]*?standard input`)
	stdOut := regexp.MustCompile(`class="output-file[^"]*"[\s\S]*?standard output`)
	if !(stdIO.Match(body) && stdOut.Match(body)) {
		standardIO = false
	}

	for i := 0; i < len(input); i++ {
		fileIn := filepath.Join(path, fmt.Sprintf("in%v.txt", i+1))
		fileOut := filepath.Join(path, fmt.Sprintf("ans%v.txt", i+1))
		e := ioutil.WriteFile(fileIn, input[i], 0644)
		if e != nil {
			if mu != nil {
				mu.Lock()
			}
			color.Red(e.Error())
			if mu != nil {
				mu.Unlock()
			}
		}
		e = ioutil.WriteFile(fileOut, output[i], 0644)
		if e != nil {
			if mu != nil {
				mu.Lock()
			}
			color.Red(e.Error())
			if mu != nil {
				mu.Unlock()
			}
		}
	}
	return len(input), standardIO, nil
}

// Parse parse
func (c *Client) Parse(info Info) (problems []string, paths []string, err error) {
	color.Cyan("Parse " + info.Hint())

	problemID := info.ProblemID
	info.ProblemID = "%v"
	urlFormatter, err := info.ProblemURL(c.host)
	if err != nil {
		return
	}
	info.ProblemID = ""
	if problemID == "" {
		statics, err := c.Statis(info)
		if err != nil {
			return nil, nil, err
		}
		problems = make([]string, len(statics))
		for i, problem := range statics {
			problems[i] = problem.ID
		}
	} else {
		problems = []string{problemID}
	}
	contestPath := info.Path()
	ansi.Printf(color.CyanString("The problem(s) will be saved to %v\n"), color.GreenString(contestPath))

	wg := sync.WaitGroup{}
	wg.Add(len(problems))
	mu := sync.Mutex{}
	paths = make([]string, len(problems))
	for i, problemID := range problems {
		paths[i] = filepath.Join(contestPath, strings.ToLower(problemID))
		go func(problemID, path string) {
			defer wg.Done()
			mu.Lock()
			fmt.Printf("Parsing %v\n", problemID)
			mu.Unlock()

			if err := os.MkdirAll(path, os.ModePerm); err != nil {
				mu.Lock()
				color.Red("Failed %v. Error: %v", problemID, err.Error())
				mu.Unlock()
				return
			}
			URL := fmt.Sprintf(urlFormatter, problemID)

			samples, standardIO, err := c.ParseProblem(URL, path, &mu)

			warns := ""
			if !standardIO {
				warns = color.YellowString("Non standard input output format.")
			}
			mu.Lock()
			if err != nil {
				color.Red("Failed %v. Error: %v", problemID, err.Error())
			} else {
				ansi.Printf("%v %v\n", color.GreenString("Parsed %v with %v samples.", problemID, samples), warns)
			}
			mu.Unlock()
		}(problemID, paths[i])
	}
	wg.Wait()
	return
}
