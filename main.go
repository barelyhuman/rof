package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const snapshotDir = ".rof_snapshots"

func main() {

	cliName := "rof"

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command>\n", cliName)
		os.Exit(1)
	}

	// Join command-line arguments into a single command string.
	command := strings.Join(os.Args[1:], " ")

	timestamp := getTimestamp()
	createSnapshots(timestamp)

	// Execute the command using the shell.
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "Failed to run command: %v\n", err)
			exitCode = 1
		}
		fmt.Fprintf(os.Stderr, "Command failed with result %d. Restoring files from snapshots...\n", exitCode)
		err = restoreSnapshots(timestamp)
		if err != nil {
			log.Fatal(err)
		}
	}

	cleanup()
	os.Exit(exitCode)
}

func getTimestamp() string {
	return time.Now().Format("20060102150405")
}

func createSnapshots(timestamp string) {
	err := os.Mkdir(snapshotDir, 0755)
	if err != nil && !os.IsExist(err) {
		fmt.Fprintf(os.Stderr, "mkdir failed: %v\n", err)
		return
	}

	// List files in the current directory.
	entries, err := os.ReadDir(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "opendir failed: %v\n", err)
		return
	}

	for _, entry := range entries {
		if entry.Type().IsRegular() {
			srcName := entry.Name()
			destName := fmt.Sprintf("%s.%s.bak", srcName, timestamp)
			destPath := filepath.Join(snapshotDir, destName)
			if err := copyFile(srcName, destPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to snapshot file %s: %v\n", srcName, err)
			}
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	_, err = io.Copy(out, in)
	return err
}

func restoreSnapshots(timestamp string) error {
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "opendir snapshot failed: %v\n", err)
		return nil
	}

	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		fname := entry.Name()
		if !strings.HasSuffix(fname, ".bak") {
			continue
		}

		// Snapshot files have the format: <orig_filename>.<timestamp>.bak
		base := fname[:len(fname)-4] // remove the ".bak" suffix
		if !strings.HasSuffix(base, timestamp) {
			return fmt.Errorf("failed to find file for timestamp %v, kindly restore it manually from the snapshot's dir %v",
				timestamp,
				snapshotDir,
			)
		}
		i := strings.LastIndex(base, ".")
		if i < 0 {
			continue // not in expected format
		}
		origName := base[:i]
		snapPath := filepath.Join(snapshotDir, fname)
		if err := copyFile(snapPath, origName); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restore file %s: %v\n", origName, err)
		} else {
			fmt.Fprintf(os.Stderr, "Restored file: %s\n", origName)
		}
	}

	return nil
}

func cleanup() {
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Snapshot directory not found: %s\n", snapshotDir)
		return
	}

	for _, entry := range entries {
		filePath := filepath.Join(snapshotDir, entry.Name())
		if entry.Type().IsRegular() {
			if err := os.Remove(filePath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to remove file: %s\n", filePath)
			}
		}
	}
	if err := os.Remove(snapshotDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to remove directory: %s\n", snapshotDir)
	}
}
