/*
   Pak: Wrapper designed for package managers to unify software management commands between distros
   Copyright (C) 2020 Arsen Musayelyan

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"flag"
	"fmt"
	"github.com/pelletier/go-toml"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
)

func main()  {
	// Put all arguments into a variable
	args := os.Args[1:]

	// Check which currentUser is running command
	currentUser, err := user.Current()
	if err != nil { log.Fatal(err) }

	// Create help flags
	var helpFlagGiven bool
	flag.BoolVar(&helpFlagGiven, "help", false, "Show help screen")
	flag.BoolVar(&helpFlagGiven, "h", false, "Show help screen (shorthand)")

	// Check to make sure root is not being used unless -r/--root specified
	var rootCheckBypass bool
	// Create --root and -r flags for root check bypass
	flag.BoolVar(&rootCheckBypass,"root", false, "Bypass root check")
	flag.BoolVar(&rootCheckBypass,"r", false, "Bypass root check (shorthand)")
	// Parse arguments for flags
	flag.Parse()

	// If flag not given
	if !rootCheckBypass {
		// If current user is root
		if strings.Contains(currentUser.Username, "root") {
			// Print warning message and exit
			fmt.Println("Do not run as root, this program will invoke root for you if selected in config.")
			fmt.Println("If you would like to bypass this, run this command with -r or --root.")
			os.Exit(1)
		}
	}

	// Create regex to remove all flags using ";;;" as it is uncommon to use in command line
	flagRegex := regexp.MustCompile(`(?m)(;;;|^)-+[^;]*;;;`)
	// Join args into string
	argsStr := strings.Join(args, ";;;")
	// Remove all flags from join args
	argsStr = flagRegex.ReplaceAllString(argsStr, "$1")
	// Separate args back into slice
	args = strings.Split(argsStr, ";;;")

	// Define variables for config file location, and override state boolean
	var configFileLocation string
	var isOverridden bool
	// Get PAK_MGR_OVERRIDE environment variable
	override := os.Getenv("PAK_MGR_OVERRIDE")
	// If override is set
	if override != "" {
		// Set configFileLocation to /etc/pak.d/{override}.cfg
		configFileLocation = "/etc/pak.d/" + override + ".cfg"
		// Set override state to true
		isOverridden = true
	} else {
		// Otherwise, set configFileLocation to default config
		configFileLocation = "/etc/pak.cfg"
		// Set override state to false
		isOverridden = false
	}

	// Parse config file removing all comments and empty lines
	config, err := ioutil.ReadFile(configFileLocation)
	parsedConfig, _ := toml.Load(string(config))

	// Set first line of config to variable
	packageManagerCommand := parsedConfig.Get("packageManager").(string)
	//fmt.Println(packageManagerCommand) //DEBUG

	// Parse list of commands in config line 2 and set to variable as array
	commands := InterfaceToString(parsedConfig.Get("commands").([]interface{}))
	//fmt.Println(commands) //DEBUG

	// Set the root option in config line 3 to a variable
	useRoot := parsedConfig.Get("useRoot").(bool)
	//fmt.Println(useRoot) //DEBUG

	// Set command to use to invoke root at config line 4 to a variable
	rootCommand := parsedConfig.Get("rootCommand").(string)
	//fmt.Println(rootCommand) //DEBUG

	// Parse list of shortcuts in config and line 5 set to variable as an array
	shortcuts := InterfaceToString(parsedConfig.Get("shortcuts").([]interface{}))
	//fmt.Println(shortcuts) //DEBUG

	// Parse list of shortcuts in config line 6 and set to variable as array
	shortcutMappings := InterfaceToString(parsedConfig.Get("shortcutMappings").([]interface{}))
	//fmt.Println(shortcutMappings) //DEBUG

	// Create similar to slice to put all matched commands into
	var similarTo []string

	// Displays help message if no arguments provided or -h/--help is passed
	if len(args) == 0 || helpFlagGiven || Contains(args, "help") {
		printHelpMessage(packageManagerCommand, useRoot, rootCommand, commands, shortcuts, shortcutMappings, isOverridden)
		os.Exit(0)
	}

	// Create distance slice to store JaroWinkler distance values
	var distance []float64
	// Appends JaroWinkler distance between each available command and the first argument to an array
	for _,command := range commands {
		distance = append(distance, JaroWinkler(command, args[0], 1, 0))
	}

	// Deals with shortcuts
	for index, shortcut := range shortcuts {
		// If the first argument is a shortcut and similarTo does not already contain its mapping, append it
		if args[0] == shortcut && !Contains(similarTo, shortcutMappings[index]) {
			similarTo = append(similarTo, shortcutMappings[index])
		}
	}

	// Compares each distance to the max of the distance slice and appends the closest command to similarTo
	for index, element := range distance {
		// If current element is the closest to the first argument
		if element == Max(distance) {
			// Append command at same index as distance to similarTo
			similarTo = append(similarTo, commands[index])
		}
	}

	// If similarTo is still empty, log it fatally as something is wrong with the config or the code
	if len(similarTo) == 0 { log.Fatalln("This command does not match any known commands or shortcuts") }
	// Anonymous function to decide whether to print (overridden)
	printOverridden := func() string { if isOverridden { return "(overridden)" } else { return "" } }
	// Print text showing command being run and package manager being used
	fmt.Println("Running:", strings.Title(similarTo[0]), "using", strings.Title(packageManagerCommand), printOverridden())
	// Run package manager with the proper arguments passed if more than one argument exists
	var cmdArr []string
	// If root is to be used, append it to cmdArr
	if useRoot { cmdArr = append(cmdArr, rootCommand) }
	// Create slice with all commands and arguments for the package manager
	cmdArr = append(cmdArr, []string{packageManagerCommand, similarTo[0]}...)
	// If greater than 2 arguments, append them to cmdArr
	if len(args) >= 2 { cmdArr = append(cmdArr, strings.Join(args[1:], " ")) }
	// Create space separated string from cmdArr
	cmdStr := strings.Join(cmdArr, " ")
	// Instantiate exec.Command object with command sh, flag -c, and cmdStr
	command := exec.Command("sh", "-c", cmdStr)
	// Set standard outputs for command
	command.Stdout = os.Stdout
	command.Stdin = os.Stdin
	command.Stderr = os.Stderr
	// Run command
	err = command.Run()
	// If command returned an error, log fatally with explanation
	if err != nil {
		fmt.Println("Error received from child process")
		log.Fatal(err)
	}
}