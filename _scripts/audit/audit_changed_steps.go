package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	collectionID    = ""
	isLocalTestMode = false
)

// -----------------------------------
// --- UTIL FUNCTIONS

func runCommandAndReturnCombinedOutputs(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	outBytes, err := cmd.CombinedOutput()
	outStr := string(outBytes)
	if err != nil {
		log.Printf(" [!] Failed to run command: %#v", cmd)
	}
	return strings.TrimSpace(outStr), err
}

func genericIsPathExists(pth string) (os.FileInfo, bool, error) {
	if pth == "" {
		return nil, false, errors.New("No path provided")
	}
	fileInf, err := os.Stat(pth)
	if err == nil {
		return nil, true, nil
	}
	if os.IsNotExist(err) {
		return fileInf, false, nil
	}
	return fileInf, false, err
}

func isPathExists(pth string) (bool, error) {
	_, isExists, err := genericIsPathExists(pth)
	return isExists, err
}

func detectStepIDAndVersionFromPath(pth string) (stepID, stepVersion string, err error) {
	pathComps := strings.Split(pth, "/")
	if len(pathComps) < 4 {
		err = fmt.Errorf("Path should contain at least 4 components: steps, step-id, step-version, step.yml: %s", pth)
		return
	}
	// we only care about the last 4 component of the path
	pathComps = pathComps[len(pathComps)-4:]
	if pathComps[0] != "steps" {
		err = fmt.Errorf("Invalid step.yml path, 'steps' should be included right before the step-id: %s", pth)
		return
	}
	if pathComps[3] != "step.yml" {
		err = fmt.Errorf("Invalid step.yml path, should end with 'step.yml': %s", pth)
		return
	}
	stepID = pathComps[1]
	stepVersion = pathComps[2]
	return
}

// normalizedOSTempDirPath ...
// Returns a temp dir path. If tmpDirNamePrefix is provided it'll be used
//  as the tmp dir's name prefix.
// Normalized: it's guaranteed that the path won't end with '/'.
func normalizedOSTempDirPath(tmpDirNamePrefix string) (retPth string, err error) {
	retPth, err = ioutil.TempDir("", tmpDirNamePrefix)
	if strings.HasSuffix(retPth, "/") {
		retPth = retPth[:len(retPth)-1]
	}
	return
}

func auditChangedStepYML(stepYmlPth string) error {
	log.Println("=> auditChangedStepYML: ", stepYmlPth)
	stepID, stepVer, err := detectStepIDAndVersionFromPath(stepYmlPth)
	if err != nil {
		return fmt.Errorf("Audit failed for (%s), error: %s", stepYmlPth, err)
	}

	log.Println("==> Step's main folder content: ")
	stepMainDirPth := "./steps/" + stepID
	lsOut, err := runCommandAndReturnCombinedOutputs("ls", "-alh", stepMainDirPth)
	log.Println()
	log.Println(lsOut)
	if err != nil {
		log.Println("Failed to 'ls -alh' the Step's main folder: ", stepMainDirPth)
		log.Println("Error: ", err)
		return err
	}
	log.Println()

	log.Println("==> Auditing step: ", stepID, " | version: ", stepVer)
	//
	tmpStepActPth, err := normalizedOSTempDirPath("" + stepID + "--" + stepVer)
	//
	output, err := runCommandAndReturnCombinedOutputs("stepman", "activate",
		"--collection", collectionID,
		"--id", stepID,
		"--version", stepVer,
		"--path", tmpStepActPth,
	)
	if err != nil {
		log.Println(" [!] Failed to run stepman activate, output was:")
		log.Println(output)
		return err
	}
	log.Println("stepman activate output: ", output)
	log.Println("==> SUCCESSFUL audit")
	return nil
}

// -----------------------------------
// --- MAIN

func init() {
	flag.StringVar(&collectionID, "collectionid", "", "Collection ID to use")
	flag.BoolVar(&isLocalTestMode, "localtest", false, "Enable local test mode - runs `git diff` on local changes instead of HEAD..origin/master")
}

func main() {
	// --- INPUTS
	flag.Parse()
	if collectionID == "" {
		log.Fatalln("Collection ID not provided!")
	}

	// --- MAIN
	log.Println("Auditing changed steps...")

	log.Println("git fetch...")
	if output, err := runCommandAndReturnCombinedOutputs("git", "fetch"); err != nil {
		log.Println(" [!] Error - Output was: ", output)
		log.Fatalln(" [!] Error: ", err)
	}

	log.Println("git diff...")
	diffOutput := ""
	var diffErr error
	//
	if isLocalTestMode {
		diffOutput, diffErr = runCommandAndReturnCombinedOutputs("git", "diff", "--name-only", "--cached", "upstream/master")
	} else {
		diffOutput, diffErr = runCommandAndReturnCombinedOutputs("git", "diff", "--name-only", "HEAD", "origin/master")
	}

	if diffErr != nil {
		log.Println(" [!] Error - Output was: ", diffOutput)
		log.Fatalln(" [!] Error: ", diffErr)
	}
	changedFilePaths := strings.Split(diffOutput, "\n")
	log.Println("Changed files: ", changedFilePaths)
	for _, aPth := range changedFilePaths {
		if strings.HasSuffix(aPth, "step.yml") {
			if isExist, err := isPathExists(aPth); err != nil {
				log.Fatalln(" [!] Failed to check path: ", aPth, " | err: ", err)
			} else if !isExist {
				log.Fatalln(" [!] Step.yml was removed: ", aPth)
			} else {
				if err := auditChangedStepYML(aPth); err != nil {
					log.Fatalf("Failed to audit step (%s), err: %s", aPth, err)
				}
			}
		} else {
			log.Println("Not a step.yml, skipping audit: ", aPth)
		}
	}

	log.Println("DONE")
}
