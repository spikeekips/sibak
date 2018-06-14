package common

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/owlchain/sebak/lib"
)

/**
 * Issue a message on Stderr then exit with an error code
 */
func PrintFlagsError(cmd *cobra.Command, flagName string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid '%s'; %v\n\n", flagName, err)
	}

	cmd.Help()

	os.Exit(1)
}

// Parse an input string as a monetary amount
//
// Commas (','), and dots ('.') and underscores ('_')
// are treated as digit separator, and not decimal separators,
// and will be skipped.
//
// Params:
//   input = the string representation of the amount, in GON
//
// Returns:
//   sebak.Amount: the value represented by `input`
//   error: an `error`, if any happened
func ParseAmountFromString(input string) (sebak.Amount, error) {
	amountStr := strings.Replace(input, ",", "", -1)
	amountStr = strings.Replace(amountStr, ".", "", -1)
	amountStr = strings.Replace(amountStr, "_", "", -1)
	return sebak.AmountFromString(amountStr)
}

func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
