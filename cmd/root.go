/*
Copyright © 2025 Izzy Lerman izzylerman14@gmail.com
*/
package cmd

import (
	"fmt"
	"funky/fnutils"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "funky",
	Short: "build and deploy serverless functions",
	Long:  `tbd`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Failed to open log file: %s", err)
		}
		defer file.Close()

		log.SetOutput(file)

		fs := &fnutils.LocalFS{}

		verbose, err := cmd.Flags().GetBool("verbose")
		if err != nil {
			fmt.Println("Error reading verbose flag:", err)
			return err
		}

		err = fnutils.GetFnStatus(fs, verbose)
		if err != nil {
			fnutils.RegisterFnService(fs, verbose)
			err = fnutils.GetFnStatus(fs, verbose)
			if err != nil {
				fmt.Println("Error: failed to start the fn daemon.\n", err, "\nExiting...")
				return err
			}
		}

		if verbose {
			fmt.Println("fn running!")
		}
		return nil

	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Funky!")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Funky",
	Long:  `All software has versions. This is Funky's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("v0.0 -- HEAD")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.funky.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Prints details about Funky's background tasks")
	rootCmd.AddCommand(versionCmd)

}
