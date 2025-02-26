// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gotest is a tiny program that shells out to `go test`
// and prints the output in color.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/fatih/color"
)

var (
	pass = color.FgGreen
	skip = color.FgYellow
	fail = color.FgHiRed

	skipnotest bool
)

const (
	paletteEnv     = "GOTEST_PALETTE"
	skipNoTestsEnv = "GOTEST_SKIPNOTESTS"
)

type ResultSummary struct {
	pass    int
	fail    int
	skipped int
}

func (r *ResultSummary) Print() {
	total := r.pass + r.fail + r.skipped
	color.Cyan("Summary:")
	color.White("Total: %d", total)
	color.Green("PASS: %d", r.pass)
	color.Yellow("SKIP: %d", r.skipped)
	color.Red("FAIL: %d", r.fail)
}

func main() {
	enablePalette()
	enableSkipNoTests()
	enableOnCI()

	os.Exit(gotest(os.Args[1:]))
}

func gotest(args []string) int {
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()

	r, w := io.Pipe()
	defer w.Close()

	args = append([]string{"test"}, args...)
	cmd := exec.Command("go", args...)
	cmd.Stderr = w
	cmd.Stdout = w
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		log.Print(err)
		wg.Done()
		return 1
	}

	go consume(&wg, r)

	sigc := make(chan os.Signal)
	done := make(chan struct{})
	defer func() {
		done <- struct{}{}
	}()
	signal.Notify(sigc)

	go func() {
		for {
			select {
			case sig := <-sigc:
				cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			return ws.ExitStatus()
		}
		return 1
	}
	return 0
}

func consume(wg *sync.WaitGroup, r io.Reader) {
	defer wg.Done()
	reader := bufio.NewReader(r)
	summary := &ResultSummary{}
	for {
		l, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Print(err)
			break
		}
		parse(string(l), summary)
	}
	summary.Print()
}

func parse(line string, summary *ResultSummary) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "=== RUN") {
		return
	}
	defer color.Unset()

	var c color.Attribute
	switch {
	case strings.Contains(trimmed, "[no test files]"):
		if skipnotest {
			return
		}

	case strings.HasPrefix(trimmed, "--- PASS"): // passed
		fallthrough
	case strings.HasPrefix(trimmed, "ok"):
		fallthrough
	case strings.HasPrefix(trimmed, "PASS"):
		summary.pass += 1
		c = pass

	// skipped
	case strings.HasPrefix(trimmed, "--- SKIP"):
		summary.skipped += 1
		c = skip

	// failed
	case strings.HasPrefix(trimmed, "--- FAIL"):
		fallthrough
	case strings.HasPrefix(trimmed, "FAIL"):
		summary.fail += 1
		c = fail
	}

	color.Set(c)
	fmt.Printf("%s\n", line)
}

func enableOnCI() {
	ci := strings.ToLower(os.Getenv("CI"))
	switch ci {
	case "true":
		fallthrough
	case "travis":
		fallthrough
	case "appveyor":
		fallthrough
	case "gitlab_ci":
		fallthrough
	case "circleci":
		color.NoColor = false
	}
}

func enablePalette() {
	v := os.Getenv(paletteEnv)
	if v == "" {
		return
	}
	vals := strings.Split(v, ",")
	if len(vals) != 2 {
		return
	}
	if c, ok := colors[vals[0]]; ok {
		fail = c
	}
	if c, ok := colors[vals[1]]; ok {
		pass = c
	}
}

func enableSkipNoTests() {
	v := os.Getenv(skipNoTestsEnv)
	if v == "" {
		return
	}
	v = strings.ToLower(v)
	skipnotest = v == "true"
}

var colors = map[string]color.Attribute{
	"black":     color.FgBlack,
	"hiblack":   color.FgHiBlack,
	"red":       color.FgRed,
	"hired":     color.FgHiRed,
	"green":     color.FgGreen,
	"higreen":   color.FgHiGreen,
	"yellow":    color.FgYellow,
	"hiyellow":  color.FgHiYellow,
	"blue":      color.FgBlue,
	"hiblue":    color.FgHiBlue,
	"magenta":   color.FgMagenta,
	"himagenta": color.FgHiMagenta,
	"cyan":      color.FgCyan,
	"hicyan":    color.FgHiCyan,
	"white":     color.FgWhite,
	"hiwhite":   color.FgHiWhite,
}
