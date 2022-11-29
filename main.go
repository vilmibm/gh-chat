package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/go-gh"
	"github.com/rivo/tview"
)

/*
	i have accepted the limitation that this command creates a room that then
	has to be transmitted out-of-band to other users. the upshot of this is
	it's easier to have more than 2 people in a chat. downside is how it's
	falsifiable, but i am ok with that for a stupid hackathon.
*/

func main() {
	client, err := gh.RESTClient(nil)
	if err != nil {
		panic(err)
	}
	response := struct{ Login string }{}
	err = client.Get("user", &response)
	if err != nil {
		panic(err)
	}
	username := response.Login

	gistOwner := username
	var gistID string
	var enter bool
	if len(os.Args) == 1 {
		// starting new chat
		// TODO actually create gist
		gistID = "b6f867cbdd5dcb3e08fca1323fae4db8"
		fmt.Printf("created chat room. tell others to run `gh chat %s/%s`\n",
			gistOwner, gistID)
		survey.AskOne(&survey.Confirm{
			Message: "continue into chat room?",
			Default: true,
		}, &enter)
	} else {
		gistID = os.Args[1]
	}
	cloneDir := path.Join(os.TempDir(), gistID)

	defer cleanup(cloneDir, gistOwner, gistID)

	if !enter {
		return
	}

	// TODO clone gist to /tmp
	gistURL := fmt.Sprintf("https://gist.github.com/%s/%s", gistOwner, gistID)
	cloneArgs := []string{"clone", gistURL, cloneDir}
	cloneCmd := exec.Command("git", cloneArgs...)
	cloneCmd.Stderr = os.Stderr
	err = cloneCmd.Run()
	if err != nil {
		panic(err)
	}

	app := tview.NewApplication()

	// TODO tview app stuff

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func cleanup(cloneDir, gistOwner, gistID string) {
	os.RemoveAll(cloneDir)
	// TODO delete gist
}

// For more examples of using go-gh, see:
// https://github.com/cli/go-gh/blob/trunk/example_gh_test.go
