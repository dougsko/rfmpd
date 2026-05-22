package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
)

var globalBroker *Broker

func main() {
	continuous := flag.Bool("continuous", false, "run continuous churn scenario")
	verbose := flag.Bool("verbose", false, "show broker traffic and node logs")
	timeout := flag.Duration("timeout", 45*time.Second, "convergence timeout per scenario")
	brokerPort := flag.Int("broker-port", 8055, "KISS broker listen port")
	churnDuration := flag.Duration("churn-duration", 60*time.Second, "duration of churn scenario")
	baudRate := flag.Int("baud", 0, "simulated baud rate (0=unlimited, 1200=typical VHF, 300=HF)")
	flag.Parse()

	rfmpdBin := findRFMPDBinary()
	if rfmpdBin == "" {
		fmt.Fprintln(os.Stderr, "ERROR: cannot find rfmpd binary. Build it first: go build -o rfmpd .")
		os.Exit(1)
	}
	fmt.Printf("Using rfmpd binary: %s\n", rfmpdBin)

	broker := NewBroker(*brokerPort, *verbose)
	globalBroker = broker
	if *baudRate > 0 {
		broker.SetBaudRate(*baudRate)
		// Scale timeout for slower channels
		baudMultiplier := 1.0
		if *baudRate <= 300 {
			baudMultiplier = 10.0
		} else if *baudRate <= 1200 {
			baudMultiplier = 5.0
		}
		*timeout = time.Duration(float64(*timeout) * baudMultiplier)
		*churnDuration = time.Duration(float64(*churnDuration) * baudMultiplier)
		scenarioBaudRate = *baudRate
		fmt.Printf("Simulating %d baud RF channel (timeout scaled %.0fx to %v)\n", *baudRate, baudMultiplier, *timeout)
	}
	if err := broker.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to start broker on port %d: %v\n", *brokerPort, err)
		os.Exit(1)
	}
	defer broker.Stop()
	fmt.Printf("KISS broker listening on port %d\n", *brokerPort)

	if *continuous {
		fmt.Println("Running continuous churn scenario...")
		result := RunScenarioChurn(*brokerPort, *churnDuration, *timeout, *verbose, rfmpdBin)
		PrintResults([]ScenarioResult{result})
		if !result.Passed {
			os.Exit(1)
		}
		return
	}

	type scenario struct {
		name string
		fn   func() ScenarioResult
	}
	scenarios := []scenario{
		{"Basic messaging", func() ScenarioResult {
			return RunScenarioBasicMessaging(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Late joiner SVEC sync", func() ScenarioResult {
			return RunScenarioLateSVEC(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Node crash and recovery", func() ScenarioResult {
			return RunScenarioCrashRecovery(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Network partition heal", func() ScenarioResult {
			return RunScenarioPartition(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Fragmentation under stress", func() ScenarioResult {
			return RunScenarioFragmentation(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Multi-client WebSocket", func() ScenarioResult {
			return RunScenarioMultiClient(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Poor RF reception", func() ScenarioResult {
			return RunScenarioPoorReception(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Large payload delivery", func() ScenarioResult {
			return RunScenarioLargePayload(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"High message volume", func() ScenarioResult {
			return RunScenarioHighVolume(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Heavy packet loss (40%)", func() ScenarioResult {
			return RunScenarioHeavyLoss(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Rapid crash cycling", func() ScenarioResult {
			return RunScenarioRapidCrashCycle(*brokerPort, *timeout, *verbose, rfmpdBin)
		}},
		{"Rapid churn", func() ScenarioResult {
			return RunScenarioChurn(*brokerPort, *churnDuration, *timeout, *verbose, rfmpdBin)
		}},
	}

	var results []ScenarioResult
	for _, s := range scenarios {
		broker.Reset()
		time.Sleep(1 * time.Second)
		fmt.Printf("Running scenario: %s...\n", s.name)
		results = append(results, s.fn())
		time.Sleep(500 * time.Millisecond)
	}

	PrintResults(results)

	for _, r := range results {
		if !r.Passed {
			os.Exit(1)
		}
	}
}

func findRFMPDBinary() string {
	// Check current directory
	if _, err := os.Stat("./rfmpd"); err == nil {
		return "./rfmpd"
	}
	// Check PATH
	path, err := exec.LookPath("rfmpd")
	if err == nil {
		return path
	}
	return ""
}
