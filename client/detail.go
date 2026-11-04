package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"gitcode.com/sheng_wang/cf-tool_modify/util"

	"github.com/PuerkitoBio/goquery"
)

// TestResult is the judge result of a single test.
type TestResult struct {
	Index       int
	Verdict     string
	Time        uint64 // milliseconds
	Memory      uint64 // bytes
	ExitCode    string
	Input       string
	Output      string
	Answer      string
	Checker     string
	Diagnostics string
	Passed      bool
}

// SubmissionDetail is the full detail of one submission.
type SubmissionDetail struct {
	ID               string
	ContestName      string
	ProblemName      string
	Author           string
	Lang             string
	Verdict          string
	Time             uint64 // milliseconds
	Memory           uint64 // bytes
	Sent             string
	Judged           string
	Tests            []TestResult
	CompilationError bool
	Source           string
	// TestsError is set when the per-test details could not be fetched, for
	// example when the submission belongs to someone else.
	TestsError error
}

var digitsReg = regexp.MustCompile(`\d+`)

func parseSize(raw string) uint64 {
	tmp := digitsReg.FindString(raw)
	if tmp == "" {
		return 0
	}
	value, _ := strconv.ParseUint(tmp, 10, 64)
	upper := strings.ToUpper(raw)
	switch {
	case strings.Contains(upper, "MB"):
		value *= 1024 * 1024
	case strings.Contains(upper, "KB"):
		value *= 1024
	}
	return value
}

func parseSubmissionDetailPage(body []byte) (d SubmissionDetail, err error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return d, err
	}

	found := false
	doc.Find("table tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		cells := row.Find("td")
		if cells.Length() < 9 {
			return true
		}
		d.ID = strings.TrimSpace(cells.Eq(0).Text())
		d.Author = strings.TrimSpace(cells.Eq(1).Text())
		d.ProblemName = strings.TrimSpace(cells.Eq(2).Text())
		d.Lang = strings.TrimSpace(cells.Eq(3).Text())
		d.Verdict = strings.TrimSpace(cells.Eq(4).Text())
		d.Time = parseSize(cells.Eq(5).Text())
		d.Memory = parseSize(cells.Eq(6).Text())
		d.Sent = strings.TrimSpace(cells.Eq(7).Text())
		d.Judged = strings.TrimSpace(cells.Eq(8).Text())
		found = true
		return false
	})

	if !found {
		return d, errors.New("Cannot parse the submission page")
	}
	return d, nil
}

func jsonString(data map[string]interface{}, key string) string {
	if value, ok := data[key]; ok {
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

func jsonBool(data map[string]interface{}, key string) bool {
	switch value := data[key].(type) {
	case bool:
		return value
	case string:
		// The endpoint reports some flags as the strings "true"/"false".
		return value == "true" || value == "1"
	}
	return false
}

func jsonUint(data map[string]interface{}, key string) uint64 {
	text := jsonString(data, key)
	if text == "" {
		return 0
	}
	value, _ := strconv.ParseUint(text, 10, 64)
	return value
}

func stripTags(text string) string {
	reg := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(reg.ReplaceAllString(text, ""))
}

func (d *SubmissionDetail) applyTests(data map[string]interface{}) {
	d.ContestName = jsonString(data, "contestName")
	if name := jsonString(data, "problemName"); name != "" {
		d.ProblemName = name
	}
	if author := stripTags(jsonString(data, "partyName")); author != "" {
		d.Author = author
	}
	if verdict := stripTags(jsonString(data, "verdict")); verdict != "" {
		d.Verdict = verdict
	}
	d.CompilationError = jsonBool(data, "compilationError")
	d.Source = jsonString(data, "source")

	count := int(jsonUint(data, "testCount"))
	for i := 1; i <= count; i++ {
		suffix := "#" + strconv.Itoa(i)
		test := TestResult{
			Index:       i,
			Verdict:     jsonString(data, "verdict"+suffix),
			Time:        jsonUint(data, "timeConsumed"+suffix),
			Memory:      jsonUint(data, "memoryConsumed"+suffix),
			ExitCode:    jsonString(data, "exitCode"+suffix),
			Input:       jsonString(data, "input"+suffix),
			Output:      jsonString(data, "output"+suffix),
			Answer:      jsonString(data, "answer"+suffix),
			Checker:     jsonString(data, "checkerStdoutAndStderr"+suffix),
			Diagnostics: jsonString(data, "diagnostics"+suffix),
			Passed:      jsonBool(data, "accepted"+suffix),
		}
		if test.Verdict == "" {
			continue
		}
		d.Tests = append(d.Tests, test)
	}
}

// postSubmitSource asks Codeforces for the per-test data of a submission.
// The endpoint only answers AJAX requests, hence the extra headers, and only
// returns data for submissions made by the logged-in user.
func (c *Client) postSubmitSource(pageURL, submissionID, csrf string) (map[string]interface{}, error) {
	form := url.Values{
		"csrf_token":   {csrf},
		"submissionId": {submissionID},
	}
	req, err := http.NewRequest(http.MethodPost, c.host+"/data/submitSource", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", pageURL)
	req.Header.Set("Origin", c.host)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if message := jsonString(data, "error"); message != "" {
		return nil, fmt.Errorf("%v (only your own submissions expose test details)", message)
	}
	return data, nil
}

// SubmissionDetail fetches the summary page and the per-test details of a
// submission. Per-test details are optional: if the endpoint refuses, the
// summary is returned with TestsError set.
func (c *Client) SubmissionDetail(info Info) (detail SubmissionDetail, err error) {
	URL, err := info.SubmissionURL(c.host)
	if err != nil {
		return
	}

	body, err := util.GetBody(c.client, URL)
	if err != nil {
		return
	}

	if bytes.Contains(body, []byte("Just a moment")) {
		return detail, errors.New("Cloudflare returned a challenge page instead of codeforces.\n" +
			"Refresh the cookies and check the browser User-Agent with `cf config`")
	}

	detail, err = parseSubmissionDetailPage(body)
	if err != nil {
		return
	}
	if detail.ID == "" {
		detail.ID = info.SubmissionID
	}

	csrf, csrfErr := findCsrf(body)
	if csrfErr != nil {
		detail.TestsError = csrfErr
		return detail, nil
	}

	data, testsErr := c.postSubmitSource(URL, info.SubmissionID, csrf)
	if testsErr != nil {
		detail.TestsError = testsErr
		return detail, nil
	}

	detail.applyTests(data)
	return detail, nil
}
