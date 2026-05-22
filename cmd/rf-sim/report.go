package main

import (
	"fmt"
	"time"
)

func PrintResults(results []ScenarioResult) {
	fmt.Println()
	fmt.Println("=== RF Protocol Simulation ===")
	fmt.Println()

	passed := 0
	for _, r := range results {
		mark := "x"
		if r.Passed {
			mark = "ok"
			passed++
		}

		line := fmt.Sprintf("  [%s] %s (%v)", mark, r.Name, r.Duration.Round(100*time.Millisecond))
		if r.Detail != "" {
			line += " - " + r.Detail
		}
		fmt.Println(line)
	}

	fmt.Println()
	if passed == len(results) {
		fmt.Printf("PASS: %d/%d scenarios\n", passed, len(results))
	} else {
		fmt.Printf("FAIL: %d/%d scenarios passed\n", passed, len(results))
	}
	fmt.Println()
}
