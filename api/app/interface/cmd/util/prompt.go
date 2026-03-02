package util

import (
	"bufio"
	"errors"

	"github.com/goark/gocli/rwi"
)

// IgnoblePromptYn asks the user whether to continue.
func IgnoblePromptYn(ui *rwi.RWI, msg string, yes bool) error {
	// If the --yes flag is set, skip the prompt.
	if yes {
		return nil
	}

	_ = ui.OutputErr(msg)
	_ = ui.OutputErr(" [y/N] ")
	scanner := bufio.NewScanner(ui.Reader())
	for scanner.Scan() {
		input := scanner.Text()
		if input == "y" || input == "Y" {
			break
		} else {
			return errors.New("")
		}
	}

	return nil
}
