package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tesseracode/tesserapatch/internal/tools/studyvalidator"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: studyvalidate <case-study-dir>")
		os.Exit(2)
	}
	report, err := studyvalidator.Validate(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	report.Sort()
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
	if !report.OK() {
		os.Exit(1)
	}
}
