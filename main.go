package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/go-gh"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

/*
	i have accepted the limitation that this command creates a room that then
	has to be transmitted out-of-band to other users. the upshot of this is
	it's easier to have more than 2 people in a chat. downside is how it's
	falsifiable, but i am ok with that for a stupid hackathon.

	ok i'm ridiculous; there is no need to use git. i can just comment on gists. there is no security at all.
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

	var gistOwner string
	var gistID string
	var enter bool
	if len(os.Args) == 1 {
		gistOwner = username
		// TODO actually create gist
		gistID = "b6f867cbdd5dcb3e08fca1323fae4db8"
		fmt.Printf("created chat room. tell others to run `gh chat %s/%s`\n",
			gistOwner, gistID)
		survey.AskOne(&survey.Confirm{
			Message: "continue into chat room?",
			Default: true,
		}, &enter)
		defer cleanupGist(gistOwner, gistID)
	} else {
		split := strings.Split(os.Args[1], "/")
		gistOwner = split[0]
		gistID = split[1]
	}

	if !enter {
		return
	}

	app := tview.NewApplication()

	msgView := tview.NewTextView()
	input := tview.NewInputField()
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		args := &struct {
			GistID string `json:"gist_id"`
			Body   string `json:"body"`
		}{
			GistID: gistID,
			Body:   input.GetText(),
		}
		response := &struct{}{}
		jargs, err := json.Marshal(args)
		if err != nil {
			panic(err)
		}

		err = client.Post(fmt.Sprintf("gists/%s/comments", gistID), bytes.NewBuffer(jargs), &response)
		if err != nil {
			panic(err)
		}
		input.SetText("")
	})

	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(msgView, 0, 10, false)
	flex.AddItem(input, 1, 1, true)

	app.SetRoot(flex, true)

	go func() {
		for true {
			// TODO poll gist comments

			time.Sleep(time.Second * 5)
		}
	}()

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func cleanupGist(gistOwner, gistID string) {
	// TODO
}
