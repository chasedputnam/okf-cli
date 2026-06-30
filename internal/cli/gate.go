package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chasedputnam/okf-cli/internal/canon/gate"
	"github.com/chasedputnam/okf-cli/internal/canon/model"
	"github.com/chasedputnam/okf-cli/internal/config"
	"github.com/chasedputnam/okf-cli/internal/sarif"
)

var gateCmd = &cobra.Command{
	Use:   "gate [store]",
	Short: "Run the Canon authority gate (validate + relationships + policy)",
	Long: `Run the unified Canon gate over a store: validate every artifact, check
relationship integrity, and classify findings as blocking or advisory per the
store's enforcement policy. Exits non-zero if any blocking finding exists.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGate,
}

func init() {
	rootCmd.AddCommand(gateCmd)
	gateCmd.Flags().Bool("json", false, "Output result as JSON")
	gateCmd.Flags().Bool("sarif", false, "Output result as SARIF 2.1.0")
}

func runGate(cmd *cobra.Command, args []string) error {
	storeRoot := "."
	if len(args) == 1 {
		storeRoot = args[0]
	}
	jsonOut, _ := cmd.Flags().GetBool("json")
	sarifOut, _ := cmd.Flags().GetBool("sarif")

	cfg, err := config.Load(storeRoot)
	if err != nil {
		return err
	}
	res, err := gate.Run(storeRoot, cfg)
	if err != nil {
		return err
	}

	switch {
	case sarifOut:
		doc := sarif.FromIssues("okf-cli", version, res.Issues)
		data, _ := json.MarshalIndent(doc, "", "  ")
		fmt.Println(string(data))
	case jsonOut:
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
	default:
		printGateText(res)
	}

	if !res.Passed() {
		os.Exit(1)
	}
	return nil
}

func printGateText(res gate.Result) {
	fmt.Println("okf-cli gate")
	fmt.Printf("Artifacts: %d\n", res.ArtifactCount)
	if res.Blocking > 0 {
		color.Red("Blocking: %d", res.Blocking)
	} else {
		fmt.Println("Blocking: 0")
	}
	if res.Advisory > 0 {
		color.Yellow("Advisory: %d", res.Advisory)
	} else {
		fmt.Println("Advisory: 0")
	}
	for _, iss := range res.Issues {
		loc := iss.Path
		if iss.Line > 0 {
			loc = fmt.Sprintf("%s:%d", iss.Path, iss.Line)
		}
		if iss.Severity == model.SeverityError {
			color.Red("  [%s] %s: %s", iss.Code, loc, iss.Message)
		} else {
			color.Yellow("  [%s] %s: %s", iss.Code, loc, iss.Message)
		}
	}
	if res.Passed() {
		color.Green("\nGate passed.")
	} else {
		color.Red("\nGate failed.")
	}
}
