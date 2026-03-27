package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nightowl/sip-test/internal/report"
	"github.com/nightowl/sip-test/internal/server"
	"github.com/nightowl/sip-test/internal/testrunner"
)

func main() {
	var (
		webAddr    = flag.String("web", ":8080", "Web UI listen address")
		configFile = flag.String("config", "", "Run tests from YAML config file (CLI mode)")
		reportDir  = flag.String("report-dir", "reports", "Directory to write reports")
		reportFmt  = flag.String("report-format", "json", "Report format: json, html, both")
	)
	flag.Parse()

	runner := testrunner.NewRunner()

	// CLI mode: run config file and exit
	if *configFile != "" {
		cfg, err := testrunner.LoadConfig(*configFile)
		if err != nil {
			log.Fatalf("Load config: %v", err)
		}

		log.Printf("Running %d test(s) from %s", len(cfg.Tests), *configFile)
		reports, err := runner.RunAll(context.Background(), cfg)
		if err != nil {
			log.Fatalf("Run tests: %v", err)
		}

		suite := report.NewSuite(*configFile, reports)

		switch *reportFmt {
		case "json":
			path, err := suite.WriteJSON(*reportDir)
			if err != nil {
				log.Fatalf("Write JSON report: %v", err)
			}
			fmt.Printf("Report written to %s\n", path)
		case "html":
			path, err := suite.WriteHTML(*reportDir)
			if err != nil {
				log.Fatalf("Write HTML report: %v", err)
			}
			fmt.Printf("Report written to %s\n", path)
		case "both":
			p1, err := suite.WriteJSON(*reportDir)
			if err != nil {
				log.Fatalf("Write JSON report: %v", err)
			}
			p2, err := suite.WriteHTML(*reportDir)
			if err != nil {
				log.Fatalf("Write HTML report: %v", err)
			}
			fmt.Printf("Reports written to %s and %s\n", p1, p2)
		}

		// Print summary
		fmt.Printf("\n--- Summary ---\n")
		fmt.Printf("Total: %d  Passed: %d  Failed: %d  Errors: %d\n",
			suite.Summary.Total, suite.Summary.Passed, suite.Summary.Failed, suite.Summary.Errors)

		if suite.Summary.Failed > 0 || suite.Summary.Errors > 0 {
			os.Exit(1)
		}
		return
	}

	// Web UI mode (default)
	log.Printf("SIP Test Tool - Web UI at http://localhost%s", *webAddr)
	srv := server.New(*webAddr, runner)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
