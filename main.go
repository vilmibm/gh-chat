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

// TODO add interrupt code to gh in a branch
// TODO consider protocol approach so things like invites can be handled accordingly
// TODO unfuck error handling lol

func main() {
	client, err := gh.RESTClient(nil)
	if err != nil {
		panic(err)
	}

	var gistID string
	var enter bool
	if len(os.Args) == 1 {
		// TODO actually create gist
		gistID = "b6f867cbdd5dcb3e08fca1323fae4db8"
		fmt.Printf("created chat room. tell others to run `gh chat %s`\n", gistID)
		survey.AskOne(&survey.Confirm{
			Message: "continue into chat room?",
			Default: true,
		}, &enter)
		defer cleanupGist(gistID)
	} else {
		split := strings.Split(os.Args[1], "/")
		gistID = split[1]
		enter = true
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
		txt := input.GetText()
		if strings.HasPrefix(txt, "/") {
			// TODO handle / commands
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
		seen := []int{}

		for true {
			resp := []struct {
				Body string
				ID   int `json:"id"`
				User struct {
					Login string
				}
			}{}
			err := client.Get(fmt.Sprintf("gists/%s/comments", gistID), &resp)
			if err != nil {
				panic(err)
			}

			for _, c := range resp {
				found := false
				for _, id := range seen {
					if c.ID == id {
						found = true
						break
					}
				}
				if found {
					continue
				}
				msgView.Write([]byte(fmt.Sprintf("%s: %s\n", c.User.Login, c.Body)))
				app.ForceDraw()
				seen = append(seen, c.ID)
			}

			time.Sleep(time.Second * 4)
		}
	}()

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func cleanupGist(gistID string) {
	// TODO delete gist
}
